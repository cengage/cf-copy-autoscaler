package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type JSONClient struct {
	HTTPClient  httpClient
	AccessToken string
}

type AutoscalerError struct {
	Description string `json:"description"`
	Errors      []struct {
		Resource string   `json:"resource"`
		Messages []string `json:"messages"`
	} `json:"errors"`
}

func (s *AutoscalerError) printError() string {
	errs := "\n"

	for _, e := range s.Errors {
		for _, m := range e.Messages {
			errs = fmt.Sprintf("%s  - %s\n", errs, m)
		}
	}

	return errs
}

func (c JSONClient) Do(method string, url string, requestData interface{}, responseData interface{}) error {
	var requestBodyReader io.Reader
	if requestData != nil {
		requestBytes, err := json.Marshal(requestData)
		if err != nil {
			return err // not tested
		}

		requestBodyReader = bytes.NewReader(requestBytes)
	}

	//fmt.Println(method, url)
	request, err := http.NewRequest(method, url, requestBodyReader)
	if err != nil {
		return err
	}

	if requestData != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	request.Header.Set("Authorization", c.AccessToken)

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		ae := AutoscalerError{}
		if err = json.NewDecoder(response.Body).Decode(&ae); err != nil {
			return fmt.Errorf("couldn't parse error response: %s", err)
		}
		return fmt.Errorf("%s", ae.printError())
	}

	if responseData != nil {
		if err = json.NewDecoder(response.Body).Decode(&responseData); err != nil {
			return fmt.Errorf("couldn't parse response: %s", err)
		}
	}

	return nil
}
