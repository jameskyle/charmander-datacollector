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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/jameskyle/pcp"
)

var configFile string
var config = Config{}

var argDbCreated = false

var PcpMetrics = []string{
	"cgroup.cpuacct.stat.user",
	"cgroup.cpuacct.stat.system",
	"cgroup.memory.usage",
	"network.interface.in.bytes",
	"network.interface.out.bytes",
	"network.interface.out.drops",
	"network.interface.in.drops",
}

type ClientCache struct {
	Client  *pcp.Client
	Metrics pcp.MetricList
}

func init() {
	//need to switch ip when deploying for prod vs local dev
	//var redisHost = flag.String("source_redis_host", "127.0.0.1:6379" "Redis IP Address:Port")
	flag.StringVar(&configFile, "config", "", "Data Collector Config File")
	flag.Parse()
	if configFile == "" {
		config.Redis.Host = "172.31.2.11"
		config.Redis.Port = 31600
		config.Influxdb.Username = "root"
		config.Influxdb.Password = "root"
		config.Influxdb.Host = "172.31.2.11"
		config.Influxdb.Port = 31410
		config.Influxdb.Database = "charmander-dc"
		config.Interval = 5
	} else {
		file, err := ioutil.ReadFile(configFile)
		if err != nil {
			glog.Errorf("Error reading config file: %s", err)
			os.Exit(1)
		}
		err = json.Unmarshal(file, &config)
		if err != nil {
			glog.Errorf("Error reading config file: %s", err)
			os.Exit(1)
		}
	}
}

func main() {
	glog.Info("Data Collector Initialization...")
	pcpport := 44323
	var contextStore = NewContext()
	var hosts = GetCadvisorHosts()
	var startTime = time.Now()
	clients := []ClientCache{}

	for len(hosts) < 1 {
		glog.Error("Could not talk to redis to obtain host, retrying in 5 seconds.")
		time.Sleep(time.Second * 5)
		hosts = GetCadvisorHosts()

		if time.Now().Sub(startTime) > 300*time.Second {
			glog.Fatal("Could not talk to redis to obtain host after 5 minutes, exiting.")
		}
	}

	for _, host := range hosts {
		endpoint := fmt.Sprintf("http://%s:%d", host, pcpport)
		context := pcp.NewContext("", "localhost")
		context.PollTimeout = 12
		client := pcp.NewClient(endpoint, context)
		client.RefreshContext()
		mquery := pcp.NewMetricQuery("")
		metrics, err := client.Metrics(mquery)
		if err != nil {
			glog.Errorf("Error fetching metrics for client: %s", err)
		}
		clients = append(clients, ClientCache{Client: client, Metrics: metrics})
	}

	for _, metricNames := range config.Influxdb.Schema {
		glog.Info("Iterating over schema")
		for _, cache := range clients {
			query := pcp.NewMetricValueQuery(metricNames, []string{})
			resp, err := cache.Client.MetricValues(query)
			if err != nil {
				glog.Errorf("Failed to retrieve metric values: %s\n", err)
			}

			for _, value := range resp.Values {
				metric := cache.Metrics.FindMetricByName(value.Name)
				indom, err := cache.Client.GetIndomForMetric(metric)
				if err != nil {
					glog.Errorf("Failed to get indom for metric: %s\n", err)
				}
				value.UpdateInstanceNames(indom)
			}
			glog.Info(resp.Values)
		}
	}
	os.Exit(0)
	contextStore.UpdateContext(hosts)

	GetInstanceMapping(contextStore)

	doWork(contextStore)
}

func doWork(contextStore *ContextList) {
	if config.Interval < 1 || config.Interval > 5 {
		config.Interval = 5
		glog.Error("Interval outside of range, using 5 seconds.")
	}

	duration := time.Duration(config.Interval)

	for host, _ := range contextStore.list {
		go func(host string, contextStore *ContextList) {
			for {
				var responseData = collectData(host, contextStore)
				if responseData.host != "" {
					processData(responseData)
				}
				time.Sleep(time.Second * duration)
			}
		}(host, contextStore)
	}

	keepAlive := make(chan int, 1)
	<-keepAlive
}
