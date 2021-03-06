package rucaptcha

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// CaptchaSolver structure
type CaptchaSolver struct {
	IsPhrase   bool
	IsRegsence bool
	IsNumeric  int
	MinLength  int
	MaxLength  int
	Language   int

	ImagePath string
	APIKey    string

	RequestURL         string
	ResultURL          string
	CheckResultTimeout time.Duration
}

// New creates instance of solver
func New(key string) *CaptchaSolver {
	return &CaptchaSolver{
		RequestURL: "http://rucaptcha.com/in.php",
		ResultURL:  "http://rucaptcha.com/res.php",
		APIKey:     key,
	}
}

func client() *http.Client {
	// proxyURL, _ := url.Parse("http://127.0.0.1:8888")
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
		// Proxy:             http.ProxyURL(proxyURL),
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   (30 * time.Second),
	}
	return client
}

// Solve get image by path and redn request to rucaptcha service
// Returns captcha code, captchaID and error if errors occured
func (solver *CaptchaSolver) SolveRecaptchaV3(key string, pageURL string, action string) (*string, *string, error) {
	captchaID, err := solver.getReCaptchaV3ID(key, pageURL, action)
	if err != nil {
		return nil, nil, err
	}

	answer, err := solver.WaitForReady(*captchaID)
	if err != nil {
		return nil, captchaID, err
	}

	return answer, captchaID, nil
}

// Solve get image by path and redn request to rucaptcha service
// Returns captcha code, captchaID and error if errors occured
func (solver *CaptchaSolver) SolveRecaptcha(key string, pageURL string) (*string, *string, error) {

	captchaID, err := solver.getReCaptchaID(key, pageURL)
	if err != nil {
		return nil, nil, err
	}

	answer, err := solver.WaitForReady(*captchaID)
	if err != nil {
		return nil, captchaID, err
	}

	return answer, captchaID, nil
}

func (solver *CaptchaSolver) Solve(path string) (*string, *string, error) {
	solver.ImagePath = path

	file, err := solver.loadCaptchaImage()
	if err != nil {
		return nil, nil, err
	}

	captchaID, err := solver.getCaptchaID(*file)
	if err != nil {
		return nil, nil, err
	}

	answer, err := solver.WaitForReady(*captchaID)
	if err != nil {
		return nil, captchaID, err
	}

	return answer, captchaID, nil
}

// Complain send complain request to service
func (solver *CaptchaSolver) Complain(captchaID string) error {
	return solver.complainRequest(captchaID)
}

func (solver *CaptchaSolver) getReCaptchaID(key string, pageURL string) (*string, error) {

	response, err := solver.sendRecaptchaRequest(key, pageURL)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(response)
	if err != nil {
		return nil, err
	}

	hasError := regexp.
		MustCompile(`ERROR`).
		MatchString(string(body))

	if hasError {
		return nil, fmt.Errorf("Captcha service error: %s\n", string(body))
	}

	isOk := regexp.
		MustCompile(`OK`).
		MatchString(string(body))

	if !isOk {
		return nil, fmt.Errorf("Unknown response: %s\n", string(body))
	}

	results := strings.Split(string(body), "|")

	return &results[1], nil
}

func (solver *CaptchaSolver) getReCaptchaV3ID(key string, pageURL string, action string) (*string, error) {

	response, err := solver.sendRecaptchaV3Request(key, pageURL, action)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(response)
	if err != nil {
		return nil, err
	}

	hasError := regexp.
		MustCompile(`ERROR`).
		MatchString(string(body))

	if hasError {
		return nil, fmt.Errorf("Captcha service error: %s\n", string(body))
	}

	isOk := regexp.
		MustCompile(`OK`).
		MatchString(string(body))

	if !isOk {
		return nil, fmt.Errorf("Unknown response: %s\n", string(body))
	}

	results := strings.Split(string(body), "|")

	return &results[1], nil
}

func (solver *CaptchaSolver) getCaptchaID(file []byte) (*string, error) {

	response, err := solver.sendRequest(file)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(response)
	if err != nil {
		return nil, err
	}

	hasError := regexp.
		MustCompile(`ERROR`).
		MatchString(string(body))

	if hasError {
		return nil, fmt.Errorf("Captcha service error: %s\n", string(body))
	}

	isOk := regexp.
		MustCompile(`OK`).
		MatchString(string(body))

	if !isOk {
		return nil, fmt.Errorf("Unknown response: %s\n", string(body))
	}

	results := strings.Split(string(body), "|")

	return &results[1], nil
}

func (solver *CaptchaSolver) WaitForReady(captchaID string) (*string, error) {

	data := url.Values{}
	data.Add("key", solver.APIKey)
	data.Add("action", "get")
	data.Add("id", captchaID)

	url := solver.ResultURL + "?"

	var response *http.Response

	defer func() {
		if response != nil {
			response.Body.Close()
		}
	}()

	var answer *string
	for {
		time.Sleep(solver.CheckResultTimeout)

		client := client()
		response, err := client.Get(url + data.Encode())
		if err != nil {
			return nil, err
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		isOk := regexp.
			MustCompile(`OK`).
			MatchString(string(body))

		if isOk {
			results := strings.Split(string(body), "|")
			answer = &results[1]
			break
		}

		notReady := regexp.
			MustCompile(`CAPCHA_NOT_READY`).
			MatchString(string(body))

		if notReady {
			continue
		}

		hasError := regexp.
			MustCompile(`ERROR`).
			MatchString(string(body))

		if hasError {
			return nil, fmt.Errorf("Error response: %s", string(body))
		}
	}

	return answer, nil
}

func (solver *CaptchaSolver) loadCaptchaImage() (*[]byte, error) {
	isHTTP := regexp.
		MustCompile(`(http://|https://)`).
		MatchString(solver.ImagePath)

	if !isHTTP {
		body, err := ioutil.ReadFile(solver.ImagePath)
		if err != nil {
			return nil, err
		}
		return &body, nil
	}

	response, err := http.Get(solver.ImagePath)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return &body, nil
}
