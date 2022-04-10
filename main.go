package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	tokenURI  = "/GenieLogin.asp"
	loginURI  = "/goform/GenieLogin"
	scrapeURI = "/DocsisStatus.asp"
)

type Modem struct {
	Url    string
	User   string
	Pass   string
	login  string
	Client *http.Client
}

type webToken struct {
	Token string
}

type dsTable struct {
	lock_status             float64
	modulation              string
	channel_id              string
	frequency               float64
	power                   float64
	snr_mer                 float64
	unerrored_codewords     float64
	correctable_codewords   float64
	uncorrectable_codewords float64
}

type usTable struct {
	lock_status float64
	modulation  string
	channel_id  string
	frequency   float64
	power       float64
}

type dsOFDMTable struct {
	lock_status                    float64
	modulation_profile_id          string
	channel_id                     string
	frequency                      float64
	power                          float64
	snr_mer                        float64
	active_subcarrier_number_range string
	unerrored_codewords            float64
	correctable_codewords          float64
	uncorrectable_codewords        float64
}

type usOFDMATable struct {
	lock_status           float64
	modulation_profile_id string
	channel_id            string
	frequency             float64
	power                 float64
}

func (modem *Modem) getToken() webToken {
	tokenURL := modem.Url + tokenURI
	client := modem.Client

	log.Println("Get request on login url " + tokenURL)

	response, err := client.Get(tokenURL)

	if err != nil {
		log.Fatalln("Error fetching response. ", err)
	}

	log.Println("Recieved response")

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Println("Error loading HTTP response body. ", err)
	}

	token, _ := document.Find("input[name='webToken']").Attr("value")

	webToken := webToken{
		Token: token,
	}

	return webToken
}

func (modem *Modem) loginFunc() {
	client := modem.Client

	log.Println("Getting webtoken")

	webToken := modem.getToken()

	log.Println("Got web token")

	loginURL := modem.Url + loginURI

	data := url.Values{
		"webToken":      {webToken.Token},
		"loginUsername": {modem.User},
		"loginPassword": {modem.Pass},
		"login":         {modem.login},
	}

	response, err := client.PostForm(loginURL, data)
	if err != nil {
		log.Println("Error posting login form", err)
	}

	defer response.Body.Close()

	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println("Error reading response to post login", err)
	}
}

func (modem *Modem) getData() *goquery.Document {
	scrapeURL := modem.Url + scrapeURI
	client := modem.Client

	response, err := client.Get(scrapeURL)

	if err != nil {
		unsuccessfulScrapes.Inc()
		log.Println("Error fetching response. ", err)
	}

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		unsuccessfulScrapes.Inc()
		log.Println("Error loading HTTP response body. ", err)
	}

	successfulScrapes.Inc()

	return document

}

var (
	successfulScrapes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "successful_modem_scrapes",
		Help: "The total number of successful modem scrapes",
	})
)

var (
	unsuccessfulScrapes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "unsuccessful_modem_scrapes",
		Help: "The total number of unsuccessful modem scrapes",
	})
)
var (
	channel_frequency = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_frequency_hz",
		Help: "Channel Frequency",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	channel_power = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_power_dbmv",
		Help: "Channel power",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	channel_snr_mer = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_snr_mer",
		Help: "Channel SNR MER",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	channel_unerrored_codewords = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_unerrored_codewords",
		Help: "The number of unerrored codewords",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	channel_correctable_codewords = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_correctable_codewords",
		Help: "The number of correctable codewords",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	channel_uncorrectable_codewords = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_uncorrectable_codewords",
		Help: "The number of uncorrectable codewords",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	channel_lock_status = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "channel_lock_status",
		Help: "The lock status of the channel",
	},
		[]string{"channel", "channel_type", "direction"})
)
var (
	systemUpTime = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_uptime_seconds",
		Help: "The system uptime in seconds",
	})
)

func convertTabletoFloat(table *goquery.Selection, i int) float64 {
	//takes a goquery selection and a row index and returns the value of the cell at that index as an integer.

	var return_value float64
	if value, err := strconv.ParseFloat(table.Find("td").Eq(i).Text(), 64); err == nil {
		return_value = value
	} else if err != nil {
		log.Fatalln(err)
	}

	return return_value
}

func convertStringTabletoFloat(table *goquery.Selection, i int, st string) float64 {
	//takes a goquery selection, row index, and string to replace and returns the value of the cell at that index as a float.
	//removes spaces before parsing

	var return_value float64

	parsed_value_with_spaces := strings.ReplaceAll(table.Find("td").Eq(i).Text(), st, "")
	parsed_value := strings.ReplaceAll(parsed_value_with_spaces, " ", "")

	if value, err := strconv.ParseFloat(parsed_value, 64); err == nil {
		return_value = value
	} else if err != nil {
		log.Fatalln(err)
	}

	return return_value
}

func convertLocktoFloat(table *goquery.Selection, i int) float64 {

	parsed_value := table.Find("td").Eq(i).Text()

	var return_value float64
	if parsed_value == "Locked" {
		return_value = 1
	} else if parsed_value == "Not Locked" {
		return_value = 0
	} else {
		log.Fatalln("Error parsing lock status")
	}

	return return_value

}

func exportMetrics(scrapeData *goquery.Document, initialRun bool) {

	//Partition Scraped Data

	downstreamTable := scrapeData.Find("table[id='dsTable']").Find("tbody").Find("tr").Slice(1, goquery.ToEnd)
	upstreamTable := scrapeData.Find("table[id='usTable']").Find("tbody").Find("tr").Slice(1, goquery.ToEnd)
	downstreamOFDMTable := scrapeData.Find("table[id='d31dsTable']").Find("tbody").Find("tr").Slice(1, goquery.ToEnd)
	upstreamOFDMATable := scrapeData.Find("table[id='d31usTable']").Find("tbody").Find("tr").Slice(1, goquery.ToEnd)
	uptimeData := scrapeData.Find("td[id='SystemUpTime']").Find("tbody").Find("b").Slice(1, goquery.ToEnd)

	log.Println("Parsing and exporting DS table data")

	downstreamTable.Each(func(i int, s *goquery.Selection) {

		dsTableData := dsTable{
			lock_status:             convertLocktoFloat(s, 1),
			modulation:              s.Find("td").Eq(2).Text(),
			channel_id:              s.Find("td").Eq(3).Text(),
			frequency:               convertStringTabletoFloat(s, 4, "Hz"),
			power:                   convertStringTabletoFloat(s, 5, "dBmV"),
			snr_mer:                 convertStringTabletoFloat(s, 6, "dB"),
			unerrored_codewords:     convertTabletoFloat(s, 7),
			correctable_codewords:   convertTabletoFloat(s, 8),
			uncorrectable_codewords: convertTabletoFloat(s, 9),
		}

		channel_lock_status.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(dsTableData.lock_status)
		channel_power.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(dsTableData.power)
		channel_snr_mer.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(dsTableData.snr_mer)
		channel_unerrored_codewords.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(dsTableData.unerrored_codewords)
		channel_correctable_codewords.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(dsTableData.correctable_codewords)
		channel_uncorrectable_codewords.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(dsTableData.uncorrectable_codewords)
		channel_frequency.With(prometheus.Labels{"channel": dsTableData.channel_id, "direction": "downstream", "channel_type": "bonded"}).Set(float64(dsTableData.frequency))

	})

	log.Println("Parsing and exporting US table data")

	upstreamTable.Each(func(i int, s *goquery.Selection) {

		usTableData := usTable{
			lock_status: convertLocktoFloat(s, 1),
			modulation:  s.Find("td").Eq(2).Text(),
			channel_id:  s.Find("td").Eq(3).Text(),
			frequency:   convertStringTabletoFloat(s, 4, "Hz"),
			power:       convertStringTabletoFloat(s, 5, "dBmV"),
		}

		channel_lock_status.With(prometheus.Labels{"channel": usTableData.channel_id, "direction": "upstream", "channel_type": "bonded"}).Set(usTableData.lock_status)
		channel_power.With(prometheus.Labels{"channel": usTableData.channel_id, "direction": "upstream", "channel_type": "bonded"}).Set(usTableData.power)
		channel_frequency.With(prometheus.Labels{"channel": usTableData.channel_id, "direction": "upstream", "channel_type": "bonded"}).Set(usTableData.frequency)

	})

	log.Println("Parsing and exporting DS OFDM table data")

	downstreamOFDMTable.Each(func(i int, s *goquery.Selection) {

		dsOFDMTableData := dsOFDMTable{
			lock_status:                    convertLocktoFloat(s, 1),
			modulation_profile_id:          s.Find("td").Eq(2).Text(),
			channel_id:                     s.Find("td").Eq(3).Text(),
			frequency:                      convertStringTabletoFloat(s, 4, "Hz"),
			power:                          convertStringTabletoFloat(s, 5, "dBmV"),
			snr_mer:                        convertStringTabletoFloat(s, 6, "dB"),
			active_subcarrier_number_range: s.Find("td").Eq(7).Text(),
			unerrored_codewords:            convertTabletoFloat(s, 8),
			correctable_codewords:          convertTabletoFloat(s, 9),
			uncorrectable_codewords:        convertTabletoFloat(s, 10),
		}

		channel_lock_status.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(dsOFDMTableData.lock_status)
		channel_power.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(dsOFDMTableData.power)
		channel_snr_mer.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(dsOFDMTableData.snr_mer)
		channel_unerrored_codewords.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(dsOFDMTableData.unerrored_codewords)
		channel_correctable_codewords.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(dsOFDMTableData.correctable_codewords)
		channel_uncorrectable_codewords.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(dsOFDMTableData.uncorrectable_codewords)
		channel_frequency.With(prometheus.Labels{"channel": dsOFDMTableData.channel_id, "direction": "downstream", "channel_type": "ofdm"}).Set(float64(dsOFDMTableData.frequency))

	})

	log.Println("Parsing and exporting US OFDMA table data")

	upstreamOFDMATable.Each(func(i int, s *goquery.Selection) {
		usOFDMATableData := usOFDMATable{
			lock_status:           convertLocktoFloat(s, 1),
			modulation_profile_id: s.Find("td").Eq(2).Text(),
			channel_id:            s.Find("td").Eq(3).Text(),
			frequency:             convertStringTabletoFloat(s, 4, "Hz"),
			power:                 convertStringTabletoFloat(s, 5, "dBmV"),
		}

		channel_lock_status.With(prometheus.Labels{"channel": usOFDMATableData.channel_id, "direction": "upstream", "channel_type": "ofdma"}).Set(usOFDMATableData.lock_status)
		channel_frequency.With(prometheus.Labels{"channel": usOFDMATableData.channel_id, "direction": "upstream", "channel_type": "ofdma"}).Set(usOFDMATableData.frequency)
		channel_power.With(prometheus.Labels{"channel": usOFDMATableData.channel_id, "direction": "upstream", "channel_type": "ofdma"}).Set(usOFDMATableData.power)
	})

	log.Println("Parsing and exporting uptime")

	var utstring string = uptimeData.Text()

	var splitString []string = strings.Split(utstring, ":")

	var hours = splitString[0]
	var hoursUnsliced = hours[0]
	var minutes = splitString[1]
	var minutesUnsliced = minutes[0]
	var seconds = splitString[2]
	var secondsUnsliced = seconds[0]

	var uptime_seconds float64 = float64(secondsUnsliced) + (float64(minutesUnsliced) * 60) + (float64(hoursUnsliced) * 3600)
	systemUpTime.Set(uptime_seconds)

}

func exporterLoop(currentModem *Modem) {
	go func() {
		var initialRun = true

		for {

			scrapeData := currentModem.getData()

			log.Println("Scraped data")
			fmt.Println(scrapeData.Selection.Text())

			var pageSelection = scrapeData.Find("function redirectPage()")

			fmt.Println("Page Selection:")
			fmt.Println(pageSelection.Text())
			fmt.Println("Page Selection length:")
			fmt.Println(strconv.Itoa(pageSelection.Length()))

			exportMetrics(scrapeData, initialRun)

			initialRun = false

			time.Sleep(time.Second * 5)
		}

	}()

}

func main() {

	log.Println("Initializing modem parameters")

	url, existsUrl := os.LookupEnv("MODEM_URL")
	user, existsUser := os.LookupEnv("MODEM_USER")
	pass, existsPass := os.LookupEnv("MODEM_PASS")
	port, existsPort := os.LookupEnv("EXPORT_PORT")

	if existsUrl {
		log.Println("Found modem url from env var")
	} else {
		url = "http://192.168.100.1"
	}

	if existsUser {
		log.Println("Found modem user from env var")
	} else {
		user = "admin"
	}

	if existsPass {
		log.Println("Found modem pass from env var")
	} else {
		pass = "password"
	}

	if existsPort {
		log.Println("Found modem port from env var")
	} else {
		port = "9527"
	}

	portstring := ":" + port

	log.Println("Initializing cookiejar")

	jar, _ := cookiejar.New(nil)

	log.Println("Initialized cookiejar")

	currentModem := Modem{
		Url:    url,
		User:   user,
		Pass:   pass,
		login:  "1",
		Client: &http.Client{Jar: jar},
	}

	// scrape modem

	log.Println("Logging in to modem")

	currentModem.loginFunc()

	log.Println("Logged in to Modem")

	exporterLoop(&currentModem)

	//initializing promhttp

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(portstring, nil))

}
