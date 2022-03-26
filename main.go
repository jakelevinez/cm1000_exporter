package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Modem struct {
	Url  string `default:"192.168.100.1"`
	User string `default:"admin"`
	Pass string `default:"password"`
}

func main() {

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9527", nil))

	currentModem := Modem{}

	url, existsUrl := os.LookupEnv("MODEM_URL")
	user, existsUser := os.LookupEnv("MODEM_USER")
	pass, existsPass := os.LookupEnv("MODEM_PASS")
	if existsUrl {
		currentModem.Url = url
	}
	if existsUser {
		currentModem.User = user
	}
	if existsPass {
		currentModem.Pass = pass
	}

}
