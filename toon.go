package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"strconv"

	"github.com/davecgh/go-spew/spew"
	"gopkg.in/yaml.v2"
)

const (
	baseUrl     = "https://toonopafstand.eneco.nl/toonMobileBackendWeb/client/"
	loginUrl    = baseUrl + "login"
	authUrl     = baseUrl + "auth/start"
	logoutUrl   = baseUrl + "auth/logout"
	stateUrl    = baseUrl + "auth/retrieveToonState"
	setPointUrl = baseUrl + "auth/setPoint"
	schemeUrl   = baseUrl + "auth/schemeState"
	stubUrl     = "http://localhost:8000"
)

type LoginForm struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LogoutForm struct {
	ClientId         string `json:"clientId"`
	ClientIdChecksum string `json:"clientIdChecksum"`
	Random           string `json:"random"`
}

type AuthForm struct {
	ClientId            int    `json:"clientId"`
	ClientIdChecksum    string `json:"clientIdChecksum"`
	AgreementId         int    `json:"agreementId"`
	AgreementIdChecksum string `json:"agreementIdChecksum"`
	Random              string `json:"random"`
}

type Agreement struct {
	AgreementId            string `json:"agreementId"`
	AgreementIdChecksum    string `json:"agreementIdChecksum"`
	City                   string `json:"city"`
	DisplayCommonName      string `json:"displayCommonName"`
	DisplayHardwareVersion string `json:"displayHardwareVersion"`
	DisplaySoftwareVersion string `json:"displaySoftwareVersion"`
	HouseNumber            string `json:"houseNumber"`
	IsToonSolar            bool   `json:"isToonSolar"`
	PostalCode             string `json:"postalCode"`
	Street                 string `json:"street"`
}

type ToonSession struct {
	Agreements       []Agreement `json:"agreements"`
	ClientId         string      `json:"clientId"`
	ClientIdChecksum string      `json:"clientIdChecksum"`
	PasswordHash     string      `json:"passwordHash"`
	Sample           bool        `json:"sample"`
	Success          bool        `json:"success"`
	Random           string      `json:"random"`
}

type ThermostatInfo struct {
	CurrentTemp            int    `json:"currentTemp"`
	CurrentSetpoint        int    `json:"currentSetpoint"`
	CurrentDisplayTemp     int    `json:"currentDisplayTemp"`
	ProgramState           int    `json:"programState"`
	ActiveState            int    `json:"activeState"`
	NextProgram            int    `json:"nextProgram"`
	NextState              int    `json:"nextState"`
	NextTime               int    `json:"nextTime"`
	NextSetpoint           int    `json:"nextSetpoint"`
	RandomConfigId         int    `json:"randomConfigId"`
	ErrorFound             int    `json:"errorFound"`
	BoilerModuleConnected  int    `json:"boilerModuleConnected"`
	RealSetpoint           int    `json:"realSetpoint"`
	BurnerInfo             string `json:"burnerInfo"`
	OtCommError            string `json:"otCommError"`
	CurrentModulationLevel int    `json:"currentModulationLevel"`
	HaveOTBoiler           int    `json:"haveOTBoiler"`
}

type ToonState struct {
	Success        bool           `json:"success"`
	ThermostatInfo ThermostatInfo `json:"thermostatInfo"`
}

func main() {
	config := flag.String("config", "", "Configuration file")
	username := flag.String("username", "", "Username")
	password := flag.String("password", "", "Password")
	getTemp := flag.Bool("temp", false, "Get current temperature in Celcius")
	getProg := flag.Bool("program", false, "Get current program state")
	getPwr := flag.Bool("power", false, "Get current power usage in Watts")
	setTemp := flag.Float64("set", 0.0, "Set temperature")

	flag.Parse()

	configfile := ""

	// If no configfile and no credentials are passed, use the default config
	// location.
	if *config == "" && (*username == "" || *password == "") {
		// check if the file exists
		usr, err := user.Current()
		if err != nil {
			log.Fatalln("Could not get current user information", err)
		}

		default_config := path.Join(usr.HomeDir, ".go-toon.conf")
		if err := CanReadFile(default_config, "default configuration"); err != nil {
			log.Fatalln(err)
		}

		configfile = default_config
	} else if *config != "" {
		// If a configfile was passed, use it.
		configfile = *config
	}

	// Use a config file if we got it.
	if configfile != "" {
		fileinfo, _ := os.Stat(configfile)
		if fileinfo.Mode().Perm().String() != "-rw-------" {
			log.Fatalln("Default config", configfile, "world-readable")
		}

		c := ReadConfig(configfile)
		*username = c["username"].(string)
		*password = c["password"].(string)
	}

	if *username == "" || *password == "" {
		log.Fatalln("Username and password required")
	}

	loginform := &LoginForm{
		Username: *username,
		Password: *password,
	}

	session := login(loginform)

	// Go through retrieval options first
	if *getTemp {
		t := getThermostatInfo(session)
		fmt.Println("Current temperature:", float64(t.CurrentTemp)/100)
		fmt.Println("Active state:", lookupState(t.ActiveState))
	}

	if *getProg {
		panic("not implemented")
	}

	if *getPwr {
		panic("not implemented")
		// getPowerUsage(session)
	}

	if *setTemp > 0.0 {
		setTemperature(session, *setTemp)
	}

	logout(session)
}

func debugResponse(r *http.Response) {
	fmt.Println("response Status:", r.Status)
	fmt.Println("response Headers:", r.Header)
	body, _ := ioutil.ReadAll(r.Body)
	// fmt.Println("response Body:", string(body))
	spew.Dump(body)
}

func getThermostatInfo(s *ToonSession) (t *ThermostatInfo) {
	state := getToonState(s)
	return &state.ThermostatInfo
}

func lookupState(state int) string {
	states := map[int]string{
		0: "comfort",
		1: "thuis",
		2: "slapen",
		3: "weg",
	}

	return states[state]
}

func setTemperature(s *ToonSession, t float64) {
	temperature := int(t * 100.0)

	req, err := http.NewRequest("GET", setPointUrl, nil)
	if err != nil {
		log.Fatalln(err)
	}

	params := req.URL.Query()
	params.Add("clientId", s.ClientId)
	params.Add("clientIdChecksum", s.ClientIdChecksum)
	params.Add("value", strconv.Itoa(temperature))
	params.Add("random", s.Random)
	req.URL.RawQuery = params.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func getToonState(s *ToonSession) (state *ToonState) {
	req, err := http.NewRequest("GET", stateUrl, nil)
	if err != nil {
		log.Fatalln(err)
	}

	params := req.URL.Query()
	params.Add("clientId", s.ClientId)
	params.Add("clientIdChecksum", s.ClientIdChecksum)
	params.Add("random", s.Random)
	req.URL.RawQuery = params.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	info := &ToonState{}
	json.Unmarshal(body, &info)

	return info
}

func login(loginform *LoginForm) (s *ToonSession) {
	req, err := http.NewRequest("GET", loginUrl, nil)
	if err != nil {
		log.Fatalln(err)
	}

	params := req.URL.Query()
	params.Add("username", loginform.Username)
	params.Add("password", loginform.Password)
	req.URL.RawQuery = params.Encode()

	// First we need to open the login page to retrieve the agreement details.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	// XXX: Should check for status/header codes (body.success)

	body, _ := ioutil.ReadAll(resp.Body)
	session := &ToonSession{}
	json.Unmarshal([]byte(body), &session)

	// Now we can actually establish the session based on the returned
	// agreement.
	req, err = http.NewRequest("GET", authUrl, nil)
	if err != nil {
		log.Fatalln(err)
	}

	params = req.URL.Query()
	params.Add("clientId", session.ClientId)
	params.Add("clientIdChecksum", session.ClientIdChecksum)
	params.Add("agreementId", session.Agreements[0].AgreementId)
	params.Add("agreementIdChecksum", session.Agreements[0].AgreementIdChecksum)
	req.URL.RawQuery = params.Encode()

	client = &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	session.Random = uuid()
	return session
}

func logout(s *ToonSession) {
	req, err := http.NewRequest("GET", logoutUrl, nil)
	if err != nil {
		log.Fatalln(err)
	}

	params := req.URL.Query()
	params.Add("clientId", s.ClientId)
	params.Add("clientIdChecksum", s.ClientIdChecksum)
	params.Add("random", s.Random)
	req.URL.RawQuery = params.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
}

func uuid() string {
	u, err := genUUID()
	if err != nil {
		log.Fatalln(err)
	}
	return u
}

func genUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

// Check if we can read the given 'path' denoting a 'what'
func CanReadFile(path string, what string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			err := fmt.Sprintf("%s %s does not exist", what, path)
			return errors.New(err)
		} else {
			err := fmt.Sprintf("Could not open %s for reading: %s", path, err)
			return errors.New(err)
		}
	}

	return nil
}

func ReadConfig(file string) map[interface{}]interface{} {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalln(err)
	}

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(data), &m)
	if err != nil {
		log.Fatalln(err)
	}

	return m
}
