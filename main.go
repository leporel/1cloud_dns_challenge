package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	apiKey  = flag.String("api", "", "api key for 1cloud.ru")
	delMode = flag.Bool("del", false, "clean up record")

	re = regexp.MustCompile(`(.*)\.([^.]+\.[^.]+)$`)
)

func main() {
	flag.Parse()

	if apiKey == nil || *apiKey == "" {
		log.Fatal("api key not provided")
	}

	certDomain := os.Getenv("CERTBOT_DOMAIN")
	subDomain := ""
	topDomain := ""

	log.Println("certDomain: ", certDomain)
	rexExp := re.FindStringSubmatch(certDomain)
	if len(rexExp) > 2 {
		topDomain = rexExp[2]

		if rexExp[1] == "*" {
			subDomain = "@"
		} else {
			subDomain = rexExp[1]
		}
	} else {
		topDomain = certDomain
		subDomain = "@"
	}
	if certDomain == "" || topDomain == "" || subDomain == "" {
		log.Fatalln("cant parse domain from env")
	}
	log.Println("top level domain: ", topDomain)
	log.Println("sub domain: ", subDomain)

	if *delMode {
		log.Printf("removing record")

		var recordID, domainID = "", ""

		f, err := os.OpenFile(certDomain+".txt", os.O_RDWR, 0755)
		if err != nil {
			log.Fatal(err)
		}

		scanner := bufio.NewReader(f)
		for {
			line, _, err := scanner.ReadLine()
			if string(line) != "" {
				rd := strings.Split(string(line), ":")
				if len(rd) > 1 {
					domainID, recordID = rd[0], rd[1]
					log.Printf("domain %v, record %v", domainID, recordID)

					if err := removeTxtRecord(domainID, recordID); err != nil {
						log.Fatalln(err)
					}
				}
			}
			if err == io.EOF {
				break
			}
		}

		if err := f.Close(); err != nil {
			log.Fatal(err)
		}

		err = os.Remove(certDomain + ".txt")
		if err != nil {
			log.Fatal(err)
		}

		return
	}
	log.Printf("creating txt record")

	domainID, err := getDomainID(topDomain)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("cloud1 domain id: ", domainID)

	acmeName := fmt.Sprintf("_acme-challenge")
	log.Printf("acme name: %s.%s", acmeName, certDomain)
	acmeToken := os.Getenv("CERTBOT_VALIDATION")
	log.Println("acme token: ", acmeToken)

	payload := map[string]string{
		"DomainId": "25017",
		"HostName": subDomain,
		"Name":     acmeName,
		"TTL":      "30",
		"Text":     acmeToken,
	}

	recordID, err := createTxtRecord(payload)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("New txt record id ", recordID)

	f, err := os.OpenFile(certDomain+".txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.WriteString(fmt.Sprintf("%v:%v\n", domainID, recordID))
	if err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

func getDomainID(name string) (int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.1cloud.ru/Dns", nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", *apiKey))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var dns = make([]DnsRecords, 0)

	err = json.Unmarshal(body, &dns)
	if err != nil {
		log.Fatalln(err)
	}

	for _, s := range dns {
		if s.Name == name {
			return s.ID, nil
		}
	}

	return 0, fmt.Errorf("not found domain in result")
}

func createTxtRecord(payload map[string]string) (int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Fatalln(err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.1cloud.ru/dns/recordtxt", bytes.NewBuffer(data))
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", *apiKey))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var record Record

	err = json.Unmarshal(body, &record)
	if err != nil {
		log.Fatalln(err)
	}

	if record.ID != 0 {
		return record.ID, nil
	}

	return 0, fmt.Errorf("not found record in result")
}

func removeTxtRecord(domainID, recordID string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://api.1cloud.ru/dns/%s/%s", domainID, recordID), nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", *apiKey))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	if resp.StatusCode == http.StatusOK {
		log.Println("record was removed")
		return nil
	}

	return fmt.Errorf("status %s", resp.StatusCode)
}

type DnsRecords struct {
	ID            int    `json:"ID"`
	Name          string `json:"Name"`
	NamePunyCode  string `json:"NamePunyCode"`
	TechName      string `json:"TechName"`
	State         string `json:"State"`
	DateCreate    string `json:"DateCreate"`
	IsDelegate    bool   `json:"IsDelegate"`
	LinkedRecords []struct {
		ID                   int    `json:"ID"`
		TypeRecord           string `json:"TypeRecord"`
		IP                   string `json:"IP"`
		HostName             string `json:"HostName"`
		Priority             string `json:"Priority"`
		Text                 string `json:"Text"`
		MnemonicName         string `json:"MnemonicName"`
		ExtHostName          string `json:"ExtHostName"`
		State                string `json:"State"`
		DateCreate           string `json:"DateCreate"`
		Service              string `json:"Service"`
		Proto                string `json:"Proto"`
		Weight               string `json:"Weight"`
		TTL                  int    `json:"TTL"`
		Port                 string `json:"Port"`
		Target               string `json:"Target"`
		CanonicalDescription string `json:"CanonicalDescription"`
		PunyName             string `json:"PunyName"`
	} `json:"LinkedRecords"`
	LinkedRecordOrdered []struct {
		ID                   int    `json:"ID"`
		TypeRecord           string `json:"TypeRecord"`
		IP                   string `json:"IP"`
		HostName             string `json:"HostName"`
		Priority             string `json:"Priority"`
		Text                 string `json:"Text"`
		MnemonicName         string `json:"MnemonicName"`
		ExtHostName          string `json:"ExtHostName"`
		State                string `json:"State"`
		DateCreate           string `json:"DateCreate"`
		Service              string `json:"Service"`
		Proto                string `json:"Proto"`
		Weight               string `json:"Weight"`
		TTL                  int    `json:"TTL"`
		Port                 string `json:"Port"`
		Target               string `json:"Target"`
		CanonicalDescription string `json:"CanonicalDescription"`
		PunyName             string `json:"PunyName"`
	} `json:"LinkedRecordOrdered"`
	CreateRecordA     interface{} `json:"CreateRecordA"`
	CreateRecordAaaa  interface{} `json:"CreateRecordAaaa"`
	CreateRecordMx    interface{} `json:"CreateRecordMx"`
	CreateRecordCname interface{} `json:"CreateRecordCname"`
	CreateRecordNs    interface{} `json:"CreateRecordNs"`
	CreateRecordTxt   interface{} `json:"CreateRecordTxt"`
	CreateRecordSrv   interface{} `json:"CreateRecordSrv"`
}

type Record struct {
	ID                   int       `json:"ID"`
	TypeRecord           string    `json:"TypeRecord"`
	IP                   string    `json:"IP"`
	HostName             string    `json:"HostName"`
	Priority             string    `json:"Priority"`
	Text                 string    `json:"Text"`
	MnemonicName         string    `json:"MnemonicName"`
	ExtHostName          string    `json:"ExtHostName"`
	State                string    `json:"State"`
	DateCreate           time.Time `json:"DateCreate"`
	Service              string    `json:"Service"`
	Proto                string    `json:"Proto"`
	Weight               string    `json:"Weight"`
	TTL                  int       `json:"TTL"`
	Port                 string    `json:"Port"`
	Target               string    `json:"Target"`
	CanonicalDescription string    `json:"CanonicalDescription"`
	PunyName             string    `json:"PunyName"`
}
