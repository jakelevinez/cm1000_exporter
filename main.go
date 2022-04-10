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

	fmt.Printf("Get request on login url " + tokenURL + "\n")

	response, err := client.Get(tokenURL)

	if err != nil {
		log.Fatalln("Error fetching response. ", err)
	}

	fmt.Printf("Got response \n")

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	token, _ := document.Find("input[name='webToken']").Attr("value")

	webToken := webToken{
		Token: token,
	}

	return webToken
}

func (modem *Modem) loginFunc() {
	client := modem.Client

	fmt.Printf("getting token \n")

	webToken := modem.getToken()

	fmt.Printf("Got web token \n")

	loginURL := modem.Url + loginURI

	data := url.Values{
		"webToken":      {webToken.Token},
		"loginUsername": {modem.User},
		"loginPassword": {modem.Pass},
		"login":         {modem.login},
	}

	fmt.Printf("%+v\n", data)

	response, err := client.PostForm(loginURL, data)

	if err != nil {
		log.Fatalln(err)
	}

	defer response.Body.Close()

	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalln(err)
	}
}

func (modem *Modem) getData() *goquery.Document {
	scrapeURL := modem.Url + scrapeURI
	client := modem.Client

	response, err := client.Get(scrapeURL)

	if err != nil {
		unsuccessfulScrapes.Inc()
		log.Fatalln("Error fetching response. ", err)
	}

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		unsuccessfulScrapes.Inc()
		log.Fatal("Error loading HTTP response body. ", err)
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
	} else if parsed_value == "Unlocked" {
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

	fmt.Printf("DS Table Data: \n")

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

	fmt.Printf("US Table Data: \n")

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

	fmt.Printf("DS OFDM Table Data:\n")

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

	fmt.Printf("US OFDMA Table Data:\n")

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
}

func exporterLoop(currentModem *Modem) {
	go func() {
		var initialRun = true

		for {

			scrapeData := currentModem.getData()

			fmt.Printf("Scraped data \n")

			exportMetrics(scrapeData, initialRun)

			initialRun = false

			time.Sleep(time.Second * 5)
		}

	}()

}

func main() {

	fmt.Printf("Initializing modem parameters \n")

	url, existsUrl := os.LookupEnv("MODEM_URL")
	user, existsUser := os.LookupEnv("MODEM_USER")
	pass, existsPass := os.LookupEnv("MODEM_PASS")
	port, existsPort := os.LookupEnv("EXPORT_PORT")

	if existsUrl {
		fmt.Printf("Found modem url from env var \n")
	} else {
		url = "http://192.168.100.1"
	}

	if existsUser {
		fmt.Printf("Found modem user from env var \n")
	} else {
		user = "admin"
	}

	if existsPass {
		fmt.Printf("Found modem pass from env var \n")
	} else {
		pass = "password"
	}

	if existsPort {
		fmt.Printf("Found modem port from env var \n")
	} else {
		port = "9527"
	}

	portstring := ":" + port

	fmt.Printf("Initializing cookiejar \n")

	jar, _ := cookiejar.New(nil)

	fmt.Printf("Initialized cookiejar \n")

	currentModem := Modem{
		Url:    url,
		User:   user,
		Pass:   pass,
		login:  "1",
		Client: &http.Client{Jar: jar},
	}

	// scrape modem

	fmt.Printf("Logging in to modem \n")

	currentModem.loginFunc()

	fmt.Printf("Logged in to Modem \n")

	exporterLoop(&currentModem)

	//initializing promhttp

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(portstring, nil))

}
