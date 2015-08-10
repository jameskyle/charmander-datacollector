package main

type Config struct {
	Interval int16
	Redis    struct {
		Host string
		Port int16
	}
	Influxdb struct {
		Host     string
		Port     int16
		Username string
		Password string
		Database string
		Schema   map[string][]string
	}
	Mesos struct {
		Port int16
	}
}
