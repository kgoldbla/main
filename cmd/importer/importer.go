package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io"
)

// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_URL            Full url + path to object. Mutually exclusive with IMPORTER_ENDPOINT
//    IMPORTER_ENDPOINT       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com
//			      			    Mutually exclusive with IMPORTER_URL
//    IMPORTER_OBJECT_PATH    Full path of object (e.g. bucket/object)
//    access and secret keys are optional. If omitted no creds are passed to the object store client
//    IMPORTER_ACCESS_KEY_ID  Optional. Access key is the user ID that uniquely identifies your account.
//    IMPORTER_SECRET_KEY     Optional. Secret key is the password to your account

const (
	IMPORTER_URL           = "IMPORTER_URL"
	IMPORTER_ENDPOINT      = "IMPORTER_ENDPOINT"
	IMPORTER_OBJECT_PATH   = "IMPORTER_OBJECT_PATH"
	IMPORTER_ACCESS_KEY_ID = "IMPORTER_ACCESS_KEY_ID"
	IMPORTER_SECRET_KEY    = "IMPORTER_SECRET_KEY"
	WRITE_PATH             = "/data"
)

type importInfo struct {
	url         string
	endpoint    string
	objectPath  string
	accessKeyId string
	secretKey   string
}

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()
	glog.Info("Starting importer")
	importInfo, err := getEnvVars()
	if err != nil {
		glog.Fatalf("main: unable to get env variables: %v", err)
	}
	dataReader, filename, err := newDataReader(importInfo)
	if err != nil {
		glog.Fatalf("main: unable to create data reader: %v", err)
	}
	glog.Infof("Beginning import of %s", filename)
	if err = streamDataToFile(dataReader, filename); err != nil {
		glog.Fatalf("main: unable to stream data to file:", err)
	}
	glog.Infof("Import complete, exiting")
}

// getEnvVars: get predefined exported env variables, perform syntax and semantic checking,
// return struct containing these vars.
func getEnvVars() (*importInfo, error) {
	url := parseEnvVar(IMPORTER_URL, false)
	ep := parseEnvVar(IMPORTER_ENDPOINT, false)
	op := parseEnvVar(IMPORTER_OBJECT_PATH, false)
	acc := parseEnvVar(IMPORTER_ACCESS_KEY_ID, false)
	sec := parseEnvVar(IMPORTER_SECRET_KEY, false)
	// check vars
	if len(ep) > 0 && len(url) > 0 {
		return nil, fmt.Errorf("getEnvVars: IMPORTER_ENDPOINT and IMPORTER_URL cannot both be defined")
	}
	if len(ep) == 0 && len(url) == 0 {
		return nil, fmt.Errorf("getEnvVars: IMPORTER_ENDPOINT or IMPORTER_URL must be defined")
	}
	if len(ep) > 0 {
		if len(op) == 0 {
			return nil, fmt.Errorf("getEnvVars: IMPORTER_OBJECT_PATH is empty")
		}
		if len(acc) == 0 || len(sec) == 0 {
			glog.Info("warn: IMPORTER_ACCESS_KEY_ID and/or IMPORTER_SECRET_KEY env variables are empty")
		}
	}
	return &importInfo{
		url:         url,
		endpoint:    ep,
		objectPath:  op,
		accessKeyId: acc,
		secretKey:   sec,
	}, nil
}

// newDataReader: given an endpoint or url return a reader and file name.
func newDataReader(importInfo *importInfo) (dataReader io.ReadCloser, filename string, err error) {
	if len(importInfo.endpoint) > 0 {
		glog.Infof("Importing data from S3 endpoint: %s", importInfo.endpoint)
		dataReader = getDataWithClient(importInfo)
		defer dataReader.Close()
		_, filename, err = parseDataPath(importInfo.objectPath, false)
		if err != nil {
			return nil, "", fmt.Errorf("newDataReader endpoint: %v", err)
		}
	} else if len(importInfo.url) > 0 {
		glog.Infof("Importing data from URL: %s", importInfo.url)
		dataReader = getDataWithHTTP(importInfo)
		defer dataReader.Close()
		_, filename, err = parseDataPath(importInfo.url, true)
		if err != nil {
			return nil, "", fmt.Errorf("newDataReader url: %v", err)
		}
	} else {
		return nil, "", fmt.Errorf("newDataReader: missing endpoint and url")
	}
	return dataReader, filename, nil
}
