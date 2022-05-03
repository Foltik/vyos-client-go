package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	url   string
	key   string
	resty *resty.Client

	mutex *sync.Mutex

	Config *ConfigService
}
type ConfigService struct{ client *Client }

func New(url string, key string) *Client {
	return NewWithClient(&http.Client{Timeout: 10 * time.Second}, url, key)
}

func NewWithClient(c *http.Client, url string, key string) *Client {
	client := &Client{
		url,
		key,
		resty.NewWithClient(c),
		&sync.Mutex{},

		nil,
	}

	client.Config = &ConfigService{client}

	return client
}

type response struct {
	Success bool
	Data    any
	Error   *string
}

// Posts a raw request with `payload` to `endpoint`.
func (c *Client) Request(endpoint string, payload any) (any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("Failed to marshal request payload.")
	}

	c.mutex.Lock()
	resp, err := c.resty.R().
		SetFormData(map[string]string{
			"key":  c.key,
			"data": string(data),
		}).
		Post(c.url + "/" + endpoint)
    c.mutex.Unlock()
	if err != nil {
		return nil, err
	}

	r := new(response)
	err = json.Unmarshal(resp.Body(), &r)
	if err != nil {
		return nil, err
	}

	// Handle errors from the API
	if r.Error != nil {
		return nil, errors.New(*r.Error)
	}

	return r.Data, err
}

// Returns the full configuration tree at the specified path
func (svc *ConfigService) ShowTree(path string) (map[string]any, error) {
	resp, err := svc.client.Request("retrieve", map[string]any{
		"op":   "showConfig",
		"path": strings.Split(path, " "),
	})
	if err != nil {
		if strings.Contains(err.Error(), "specified path is empty") {
			// If we get an empty path error, consume it and return nil
			return nil, nil
		} else {
			return nil, err
		}
	}

	obj, ok := resp.(map[string]any)
	if !ok {
		return nil, errors.New("Received unexpected repsonse format from server.")
	}

	return obj, nil
}

// Returns the single configuration value at the speicfied path
func (svc *ConfigService) Show(path string) (*string, error) {
	obj, err := svc.ShowTree(path)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}

	components := strings.Split(path, " ")
	terminal := components[len(components)-1]

	val, ok := obj[terminal].(string)
	if !ok {
		return nil, errors.New("Value missing from configuration tree returned by server.")
	}

	return &val, nil
}

// Sets a configuration value at the specified path
func (svc *ConfigService) Set(path string, value string) error {
	_, err := svc.client.Request("configure", map[string]any{
		"op":    "set",
		"path":  strings.Split(path, " "),
		"value": value,
	})
	return err
}

// Deletes the configuration tree/values at the specified paths
func (svc *ConfigService) Delete(paths ...string) error {
	data := []map[string]any{}
	for _, path := range paths {
		data = append(data, map[string]any{
			"op":   "delete",
			"path": strings.Split(path, " "),
		})
	}

	_, err := svc.client.Request("configure", data)
	return err
}

func flatten(result *[][]string, value any, path string) error {
	switch value.(type) {
	case map[string]any:
		tree := value.(map[string]any)

		if len(tree) == 0 {
			*result = append(*result, []string{path, ""})
		}

		for k, v := range tree {
			subpath := path
			if len(subpath) > 0 {
				subpath += " "
			}
			subpath += k

			err := flatten(result, v, subpath)
			if err != nil {
				return err
			}
		}

	case []string:
		array := value.([]string)

		if len(array) == 0 {
			*result = append(*result, []string{path, ""})
		}

		for _, v := range array {
			err := flatten(result, v, path)
			if err != nil {
				return err
			}
		}

	case string:
		*result = append(*result, []string{path, value.(string)})

	default:
		return fmt.Errorf("%s: Invalid type %T", path, value)
	}

	return nil
}

// Flatten a multi level object into a flat list of {key, value} pairs
func Flatten(tree map[string]any) ([][]string, error) {
	res := [][]string{}
	err := flatten(&res, tree, "")
	return res, err
}

// Sets all of the configuration keys and values in an object
func (svc *ConfigService) SetTree(tree map[string]any) error {
	flat, err := Flatten(tree)
	if err != nil {
		return err
	}

	data := []map[string]any{}
	for _, pair := range flat {
		path, value := pair[0], pair[1]
		data = append(data, map[string]any{
			"op":    "set",
			"path":  strings.Split(path, " "),
			"value": value,
		})
	}

	_, err = svc.client.Request("configure", data)
	return err
}

// Deletes all of the configuration keys in an object
func (svc *ConfigService) DeleteTree(tree map[string]any) error {
	flat, err := Flatten(tree)
	if err != nil {
		return err
	}

	data := []map[string]any{}
	for _, pair := range flat {
		path, value := pair[0], pair[1]

		target := path
		if value != "" {
			target += " " + value
		}

		data = append(data, map[string]any{
			"op":   "delete",
			"path": strings.Split(target, " "),
		})
	}

	_, err = svc.client.Request("configure", data)
	return err
}
