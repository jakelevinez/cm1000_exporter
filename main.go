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
	channel               int
	lock_status           string
	modulation_profile_id string
	channel_id            int
	frequency             int
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
		log.Fatalln("Error fetching response. ", err)
	}

	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	return document

}

func convertTabletoI(table *goquery.Selection, i int) int {
	//takes a goquery selection and a row index and returns the value of the cell at that index as an integer.

	var return_value int
	if value, err := strconv.Atoi(table.Find("td").Eq(i).Text()); err == nil {
		return_value = value
	} else if err != nil {
		log.Fatalln(err)
	}

	return return_value
}

func convertStringTabletoI(table *goquery.Selection, i int, st string) int {
	//takes a goquery selection, row index, and string to replace and returns the value of the cell at that index as an integer.
	//removes spaces before parsing

	var return_value int

	parsed_value_with_spaces := strings.ReplaceAll(table.Find("td").Eq(i).Text(), st, "")
	parsed_value := strings.ReplaceAll(parsed_value_with_spaces, " ", "")

	if value, err := strconv.Atoi(parsed_value); err == nil {
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
	downstreamOFDMTable := ScrapeData.Find("table[id='d31dsTable']")
	upstreamOFDMATable := ScrapeData.Find("table[id='d31usTable']")

	//fmt.Printf(downstreamTable.Text())
	// fmt.Printf(upstreamTable.Text())
	// fmt.Printf(dsOFDMTable.Text())
	// fmt.Printf(usOFDMATable.Text())
	// fmt.Printf(downstreamTable.Text())
	// fmt.Printf("Printed DS table \n")

	fmt.Printf("DS Table Data: \n")

	downstreamTable.Each(func(i int, s *goquery.Selection) {

		dsTableData := dsTable{
			channel:                 convertTabletoI(s, 0),
			lock_status:             s.Find("td").Eq(1).Text(),
			modulation:              s.Find("td").Eq(2).Text(),
			channel_id:              convertTabletoI(s, 3),
			frequency:               convertStringTabletoI(s, 4, "Hz"),
			power:                   convertStringTabletoFloat(s, 5, "dBmV"),
			snr_mer:                 convertStringTabletoFloat(s, 6, "dB"),
			unerrored_codewords:     convertTabletoI(s, 7),
			correctable_codewords:   convertTabletoI(s, 8),
			uncorrectable_codewords: convertTabletoI(s, 9),
		}

		fmt.Printf("%v \n", dsTableData)

	})

	fmt.Printf("US Table Data: \n")

	upstreamTable.Each(func(i int, s *goquery.Selection) {

		usTableData := usTable{
			channel:     convertTabletoI(s, 0),
			lock_status: s.Find("td").Eq(1).Text(),
			modulation:  s.Find("td").Eq(2).Text(),
			channel_id:  convertTabletoI(s, 3),
			frequency:   convertStringTabletoI(s, 4, "Hz"),
			power:       convertStringTabletoFloat(s, 5, "dBmV"),
		}

		fmt.Printf("%v \n", usTableData)

	})

	fmt.Printf("DS OFDM Table Data:\n")

	downstreamOFDMTable.Each(func(i int, s *goquery.Selection) {

		dsOFDMTableData := dsOFDMTable{
			channel:                        convertTabletoI(s, 0),
			lock_status:                    s.Find("td").Eq(1).Text(),
			modulation_profile_id:          s.Find("td").Eq(2).Text(),
			channel_id:                     convertTabletoI(s, 3),
			frequency:                      convertStringTabletoI(s, 4, "Hz"),
			power:                          convertStringTabletoFloat(s, 5, "dBmV"),
			snr_mer:                        convertStringTabletoFloat(s, 6, "dB"),
			active_subcarrier_number_range: s.Find("td").Eq(7).Text(),
			unerrored_codewords:            convertTabletoI(s, 8),
			correctable_codewords:          convertTabletoI(s, 9),
			uncorrectable_codewords:        convertTabletoI(s, 10),
		}

		fmt.Printf("%v \n", dsOFDMTableData)

	})

	fmt.Printf("US OFDMA Table Data:\n")

	upstreamOFDMATable.Each(func(i int, s *goquery.Selection) {
		usOFDMATableData := usOFDMATable{
			channel:               convertTabletoI(s, 0),
			lock_status:           s.Find("td").Eq(1).Text(),
			modulation_profile_id: s.Find("td").Eq(2).Text(),
			channel_id:            convertTabletoI(s, 3),
			frequency:             convertStringTabletoI(s, 4, "Hz"),
			power:                 convertStringTabletoFloat(s, 5, "dBmV"),
		}

		fmt.Printf("%v \n", usOFDMATableData)
	})

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(port, nil))

}
