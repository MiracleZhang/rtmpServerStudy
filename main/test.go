package main

import (
	"github.com/MiracleZhang/rtmpServerStudy/rtmp"
	"fmt"
	_ "net/http/pprof"
	"flag"
	"os"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var GConfFile string
var GDefaultPath string
var (
	version = "1.0.0.0"
)

func prometheus() {
	http.Handle("/metrics", promhttp.Handler())
	addr:=":9090"
	http.ListenAndServe(addr, nil)
}


func ParseCommandLine() {

	ok := flag.Bool("v", false, "is ok")
	flag.StringVar(&GConfFile, "c", "config.yaml", "General configuration file of rtmpserver")

	flag.StringVar(&GDefaultPath, "p", "/usr/local/rtmpserver/", "Default file path of rtmpserver")

	if GDefaultPath[len(GDefaultPath)-1] != '/' {
		GDefaultPath = GDefaultPath + "/"
	}

	flag.Parse()
	if *ok == true {
		fmt.Println(version)
		os.Exit(1)
	}
}

// obs push
// ffplay.exe 'rtmp://127.0.0.1/live?vhost=test.live.com/1231'
//./main -c config.yaml -p ./ >1 &
func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU() - 1)
	ParseCommandLine()
	go prometheus()
	confFile := fmt.Sprintf("%s%s", GDefaultPath, GConfFile)
	if err,srv:=rtmp.NewServer(confFile);err != nil {
		return
	}else{
		srv.ListenAndServersStart()
	}
}
