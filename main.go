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

	"github.com/PuerkitoBio/goquery"
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
	channel                 int
	lock_status             string
	modulation              string
	channel_id              int
	frequency               int
	power                   float64
	snr_mer                 float64
	unerrored_codewords     int
	correctable_codewords   int
	uncorrectable_codewords int
}

type usTable struct {
	channel     int
	lock_status string
	modulation  string
	channel_id  int
	frequency   int
	power       float64
}

type dsOFDMTable struct {
	channel                        int
	lock_status                    string
	modulation_profile_id          string
	channel_id                     int
	frequency                      int
	power                          float64
	snr_mer                        float64
	active_subcarrier_number_range string
	unerrored_codewords            int
	correctable_codewords          int
	uncorrectable_codewords        int
}

type usOFDMATable struct {
	lock_status string
	modulation  string
	channel_id  int
	frequency   string
	power       string
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
		log.Fatalln("Error fetching response. ", err)
	}

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	return document

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

	ScrapeData := currentModem.getData()

	fmt.Printf("Scraped data \n")

	//Partition Scraped Data

	downstreamTable := ScrapeData.Find("table[id='dsTable']").Find("tbody").Find("tr").Slice(1, goquery.ToEnd)
	upstreamTable := ScrapeData.Find("table[id='usTable']").Find("tbody").Find("tr").Slice(1, goquery.ToEnd)
	// downstreamOFDMTable := ScrapeData.Find("table[id='d31dsTable']")
	// upstreamOFDMATable := ScrapeData.Find("table[id='d31usTable']")

	//fmt.Printf(downstreamTable.Text())
	// fmt.Printf(upstreamTable.Text())
	// fmt.Printf(dsOFDMTable.Text())
	// fmt.Printf(usOFDMATable.Text())
	// fmt.Printf(downstreamTable.Text())
	// fmt.Printf("Printed DS table \n")

	downstreamTable.Each(func(i int, s *goquery.Selection) {
		var channel_value int
		var channel_id_value int
		var frequency_value int
		var power_value float64
		var snr_mer_value float64
		var unerrored_codewords_value int
		var correctable_codewords_value int
		var uncorrectable_codewords_value int

		if value, err := strconv.Atoi(s.Find("td").Eq(0).Text()); err == nil {
			channel_value = value
		}
		if value, err := strconv.Atoi(s.Find("td").Eq(3).Text()); err == nil {
			channel_id_value = value
		}

		parsed_frequency := strings.ReplaceAll(s.Find("td").Eq(4).Text(), "Hz", "")

		if value, err := strconv.Atoi(parsed_frequency); err == nil {
			frequency_value = value
		}

		parsed_power := strings.ReplaceAll(s.Find("td").Eq(5).Text(), "dBmV", "")

		if value, err := strconv.ParseFloat(parsed_power, 64); err == nil {
			power_value = value
		}

		parsed_snr_mer := strings.ReplaceAll(s.Find("td").Eq(6).Text(), "dB", "")

		if value, err := strconv.ParseFloat(parsed_snr_mer, 64); err == nil {
			snr_mer_value = value
		}

		parsed_unerrored_codewords := strings.ReplaceAll(s.Find("td").Eq(7).Text(), "", "")

		if value, err := strconv.Atoi(parsed_unerrored_codewords); err == nil {
			unerrored_codewords_value = value
		}

		parsed_correctable_codewords := strings.ReplaceAll(s.Find("td").Eq(8).Text(), "", "")

		if value, err := strconv.Atoi(parsed_correctable_codewords); err == nil {
			correctable_codewords_value = value
		}

		parsed_uncorrectable_codewords := strings.ReplaceAll(s.Find("td").Eq(9).Text(), "", "")

		if value, err := strconv.Atoi(parsed_uncorrectable_codewords); err == nil {
			uncorrectable_codewords_value = value
		}

		dsTableData := dsTable{
			channel:                 channel_value,
			lock_status:             s.Find("td").Eq(1).Text(),
			modulation:              s.Find("td").Eq(2).Text(),
			channel_id:              channel_id_value,
			frequency:               frequency_value,
			power:                   power_value,
			snr_mer:                 snr_mer_value,
			unerrored_codewords:     unerrored_codewords_value,
			correctable_codewords:   correctable_codewords_value,
			uncorrectable_codewords: uncorrectable_codewords_value,
		}

		fmt.Printf("DS Table Data: %v \n", dsTableData)

	})

	upstreamTable.Each(func(i int, s *goquery.Selection) {
		var channel_value int
		var channel_id_value int
		var frequency_value int
		var power_value float64

		if value, err := strconv.Atoi(s.Find("td").Eq(0).Text()); err == nil {
			channel_value = value
		}

		if value, err := strconv.Atoi(s.Find("td").Eq(3).Text()); err == nil {
			channel_id_value = value
		}

		parsed_frequency := strings.ReplaceAll(s.Find("td").Eq(4).Text(), "Hz", "")

		if value, err := strconv.Atoi(parsed_frequency); err == nil {
			frequency_value = value
		}

		parsed_power := strings.ReplaceAll(s.Find("td").Eq(5).Text(), "dBmV", "")

		if value, err := strconv.ParseFloat(parsed_power, 64); err == nil {
			power_value = value
		}

		usTableData := usTable{
			channel:     channel_value,
			lock_status: s.Find("td").Eq(1).Text(),
			modulation:  s.Find("td").Eq(2).Text(),
			channel_id:  channel_id_value,
			frequency:   frequency_value,
			power:       power_value,
		}

		fmt.Printf("US Table Data: %v \n", usTableData)

	})

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(port, nil))

}
