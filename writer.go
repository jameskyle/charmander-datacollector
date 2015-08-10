// The MIT License (MIT)
//
// Copyright (c) 2014 AT&T
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/influxdb/influxdb/client"
)

var Machine = []string{
	"time",
	"hostname",
	"memory_usage",
	"cpu_usage_system",
	"cpu_usage_user",
}

var Stats = []string{
	"time",
	"memory_usage",
	"hostname",
	"container_name",
	"cpu_usage_user",
	"cpu_usage_system",
}

var Network = []string{
	"time",
	"hostname",
	"network_in_bytes",
	"network_out_bytes",
	"interface_name",
	"network_in_drops",
	"network_out_drops",
}

func Write(data [][]interface{}, dataType string) bool {
	endpoint := fmt.Sprintf("%s:%d", config.Influxdb.Host, config.Influxdb.Port)
	c, err := client.NewClient(&client.ClientConfig{
		Host:     endpoint,
		Username: config.Influxdb.Username,
		Password: config.Influxdb.Password,
		Database: config.Influxdb.Database,
	})

	if err != nil {
		panic(err)
	}

	if argDbCreated == false {
		argDbCreated = true
		if err := c.CreateDatabase(config.Influxdb.Database); err != nil {
			glog.Errorf("Database creation failed with: %s", err)
		} else {
			glog.Info("Creating Database...")
		}
	}

	c.DisableCompression()

	var column []string

	switch dataType {
	case "machine":
		column = Machine
	case "stats":
		column = Stats
	case "network":
		column = Network
	default:
		glog.Error("Unrecognized database")
		return false
	}

	series := &client.Series{
		Name:    dataType,
		Columns: column,
		Points:  data,
	}
	glog.Infoln(series)

	if err := c.WriteSeriesWithTimePrecision([]*client.Series{series}, client.Second); err != nil {
		glog.Errorln("Failed to write", dataType, "to influxDb.", err)
		glog.Errorln("Data:", series)
		if strings.Contains(err.Error(), "400") {
			argDbCreated = false
		}
		return false
	}

	return true
}
