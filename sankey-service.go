package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/microlib/simple"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	logger simple.Logger
	config Config
)

type Sankey struct {
	I     int64   `json:"1"`
	From  string  `json:"From"`
	To    string  `json:"To"`
	Count float64 `json:"Count"`
}

type SchemaInterface struct {
	Target     string   `json:"target"`
	DataPoints []Sankey `json:"datapoints"`
}

func startHttpServer(cfg Config) *http.Server {

	config = cfg

	logger.Debug(fmt.Sprintf("Config in startServer %v ", config))
	srv := &http.Server{Addr: ":" + config.Port}

	r := mux.NewRouter()
	r.HandleFunc("/", IsAlive).Methods("GET")
	r.HandleFunc("/search", getAwsData).Methods("POST")
	r.HandleFunc("/query", getAwsData).Methods("POST")
	r.HandleFunc("/annotation", getAwsData).Methods("POST")
	http.Handle("/", r)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("Httpserver: ListenAndServe() error: " + err.Error())
		}
	}()

	return srv
}

func main() {
	// read the config
	config, _ := Init("config.json")
	logger.Level = config.Level
	srv := startHttpServer(config)
	logger.Info("Starting server on port " + config.Port)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	exit_chan := make(chan int)

	go func() {
		for {
			s := <-c
			switch s {
			case syscall.SIGHUP:
				exit_chan <- 0
			case syscall.SIGINT:
				exit_chan <- 0
			case syscall.SIGTERM:
				exit_chan <- 0
			case syscall.SIGQUIT:
				exit_chan <- 0
			default:
				exit_chan <- 1
			}
		}
	}()

	code := <-exit_chan

	if err := srv.Shutdown(nil); err != nil {
		panic(err)
	}
	logger.Info("Server shutdown successfully")
	os.Exit(code)
}

func getAwsData(w http.ResponseWriter, r *http.Request) {

	logger.Trace("In function getAwsData")

	var j map[string]interface{}
	var f []interface{}
	var sankeys []Sankey
	jsonBody, _ := ioutil.ReadAll(r.Body)

	e := json.Unmarshal(jsonBody, &j)
	if e != nil {
		logger.Error(fmt.Sprintf("getAwsData %v\n", e.Error()))
	}
	logger.Debug(fmt.Sprintf("json data %v\n", j))

	// use this for cors
	//res.setHeader("Access-Control-Allow-Origin", "*")
	//res.setHeader("Access-Control-Allow-Methods", "POST")
	//res.setHeader("Access-Control-Allow-Headers", "accept, content-type")

	// set up http object
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET", config.Url, nil)
	req.Header.Set("X-Api-Key", "")
	resp, err := client.Do(req)

	checkError(err)
	logger.Info(fmt.Sprintf("Connected to host %s", config.Url))
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	checkError(err)
	errs := json.Unmarshal(body, &f)
	checkError(errs)

	// we assume the returned json is an array of object
	for i, _ := range f {
		// each object should be in the form [from, to, count]
		var hld []interface{}
		hld = f[i].([]interface{})
		// also add in a time stamp
		threshold, _ := strconv.ParseFloat(config.Threshold, 64)
		if hld[2].(float64) > threshold {
			sankey := Sankey{I: time.Now().Unix(), From: hld[0].(string), To: hld[1].(string), Count: hld[2].(float64)}
			sankeys = append(sankeys, sankey)
		}
	}

	schema := SchemaInterface{Target: "Test", DataPoints: sankeys}
	logger.Trace(fmt.Sprintf("Handling schema %v ", schema))

	if err != nil || errs != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error in server")
	} else {
		var schemas []SchemaInterface
		schemas = append(schemas, schema)
		b, _ := json.MarshalIndent(schemas, "", "	")
		fmt.Fprintf(w, string(b))
	}

}

func checkError(err error) {
	if err != nil {
		//log.Fatal("Fatal error: %s", err.Error())
		logger.Error(fmt.Sprintf("%s ", err.Error()))
	}
}

// IsAlive a http response and request wrapper for health endpoint checks
// It takes a both response and request objects and returns void
func IsAlive(w http.ResponseWriter, r *http.Request) {
	logger.Trace(fmt.Sprintf("used to mask cc %v", r))
	//logger.Trace(fmt.Sprintf("config data  %v", config))
	fmt.Fprintf(w, "ok version 1.0")
}
