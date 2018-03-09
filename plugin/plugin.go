package plugin

import (
	"fmt"
	"os"
	"errors"
	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/cli/plugin/models"
	"net/http"
	"crypto/tls"
	"net/url"
	"encoding/json"
	"io/ioutil"
)

func NewPlugin() *Plugin {
	return &Plugin{}
}

type Plugin struct{}



func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stdout, "error:", err)
		os.Exit(1)
	}
}

type SaveFile struct{
	Rules    Rules       `json:"rules"`
	Schedule Schedule    `json:"schedule"`
}

func (s *SaveFile) printJSON() (error) {
	jsonString, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", jsonString)

	return nil
}



func (s *SaveFile) load(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &s); err != nil {
		panic(err)
	}

	return nil
}

func (s *SaveFile) save(filename string) error {
	jsonString, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, jsonString, 0644)
	if err != nil {
		return err
	}

	return nil
}

type CopyAutoscale struct{}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type jsonClient interface {
	Do(method string, url string, requestData interface{}, responseData interface{}) error
}

type CLIDependencies struct {
	AccessToken string
	AppName     string
	ServiceName string
	Service     plugin_models.GetService_Model
	APIEndpoint string
	App         plugin_models.GetAppModel
	JSONClient  jsonClient
	FileName    string
	Method      string
}

type cliConnection interface {
	IsLoggedIn() (bool, error)
	AccessToken() (string, error)
	ApiEndpoint() (string, error)
	GetService(name string) (plugin_models.GetService_Model, error)
	GetApp(name string) (plugin_models.GetAppModel, error)
	IsSSLDisabled() (bool, error)
}

func findAutoscaler(services []plugin_models.GetServices_Model, err error) (string, error) {

	if(err != nil){
		return "", err
	}

	vsf := make([]string, 0)
	for _, v := range services {

		if v.Service.Name == "app-autoscaler" {
			vsf = append(vsf, v.Name)
		}
	}

	if len(vsf) != 1 {
		return "", ErrNoAppScaler
	}

	return vsf[0], nil
}

func getBindingURL(fullDashboardURL, bindingGUID string) (string, error) {
	dashboardURL, err := url.Parse(fullDashboardURL)
	if err != nil {
		return "", fmt.Errorf("invalid dashboard URL from service instance: %s", fullDashboardURL)
	}

	baseURL := fmt.Sprintf("%s://%s", dashboardURL.Scheme, dashboardURL.Host)
	return fmt.Sprintf("%s/api/bindings/%s", baseURL, bindingGUID), nil
}

func getScheduleURL(bindingURL string) (string) {
	return fmt.Sprintf("%s/scheduled_limit_changes", bindingURL)
}

func getCCQueryURL(apiEndpoint, appGUID, serviceInstanceGUID string) (string, error) {
	serviceBindingsURL, err := url.Parse(apiEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid API URL from cli: %s", apiEndpoint)
	}

	serviceBindingsURL.Path = "/v2/service_bindings"
	serviceBindingsURL.RawQuery = url.Values{
		"q": []string{
			fmt.Sprintf("app_guid:%s", appGUID),
			fmt.Sprintf("service_instance_guid:%s", serviceInstanceGUID),
		},
	}.Encode()

	return serviceBindingsURL.String(), nil
}

var (
	ErrNoArgs     = errors.New("app name must be specified")
	ErrNoPaths    = errors.New("a config file is required to be loaded or saved")
	ErrBothPaths  = errors.New("a config file cannot be both loaded and saved")
	ErrNoAppScaler  = errors.New("an autoscaler service cannot be found")
)

func (p *Plugin) FetchCLIDependencies(cliConnection plugin.CliConnection, args []string) (CLIDependencies, error) {


	if len(args) != 3 {
		return CLIDependencies{}, fmt.Errorf("invalid parameters")
	}

	appName  := args[0]
	method   := args[1]
	fileName := args[2]

	if method != "import" && method != "export" {
		return CLIDependencies{}, fmt.Errorf("method must be 'import' or 'export', not: %s", method)
	}

	fmt.Printf("%sing %s for %s\n\n", method, fileName, appName)


	isLoggedIn, err := cliConnection.IsLoggedIn()
	if err != nil {
		return CLIDependencies{}, err
	}
	if !isLoggedIn {
		return CLIDependencies{}, fmt.Errorf("you need to log in")
	}

	accessToken, err := cliConnection.AccessToken()
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get access token: %s", err)
	}

	apiEndpoint, err := cliConnection.ApiEndpoint()
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get API end-point: %s", err)
	}

	app, err := cliConnection.GetApp(appName)
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get app %s: %s", appName, err)
	}

	serviceName, err := findAutoscaler(cliConnection.GetServices())
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get app %s: %s", appName, err)
	}

	service, err := cliConnection.GetService(serviceName)
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't get service named %s: %s", serviceName, err)
	}

	skipVerifySSL, err := cliConnection.IsSSLDisabled()
	if err != nil {
		return CLIDependencies{}, fmt.Errorf("couldn't check if ssl verification is disabled: %s", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerifySSL,
			},
		},
	}

	jsonClient := &JSONClient{
		HTTPClient:  httpClient,
		AccessToken: accessToken,
	}

	return CLIDependencies{
		AccessToken: accessToken,
		AppName:     appName,
		ServiceName: serviceName,
		Service:     service,
		APIEndpoint: apiEndpoint,
		App:         app,
		JSONClient:  jsonClient,
		FileName:    fileName,
		Method:      method,
	}, nil
}

func (p *Plugin) RunWithError(dependencies CLIDependencies) error {
	appGUID := dependencies.App.Guid

	// get from cloud controller
	serviceBindingsURL, err := getCCQueryURL(dependencies.APIEndpoint, appGUID, dependencies.Service.Guid)
	if err != nil {
		return err
	}

	var ccResponse struct {
		Resources []struct {
			Metadata struct {
				GUID string
			}
		}
	}

	err = dependencies.JSONClient.Do("GET", serviceBindingsURL, nil, &ccResponse)
	if err != nil {
		return fmt.Errorf("couldn't retrieve service binding: %s", err)
	}

	if len(ccResponse.Resources) != 1 {
		return fmt.Errorf("couldn't find service binding for %s to %s", dependencies.AppName, dependencies.ServiceName)
	}

	// get from autoscaling
	fullURL, err := getBindingURL(dependencies.Service.DashboardUrl, ccResponse.Resources[0].Metadata.GUID)
	if err != nil {
		return err
	}

	scheduleURL := getScheduleURL(fullURL)

	if dependencies.Method == "export" {
		// Get Rules from autoscaling
		rules := Rules{}
		err = dependencies.JSONClient.Do("GET", fullURL, nil, &rules)
		if err != nil {
			return fmt.Errorf("autoscaling API: %s", err)
		}

		rules.clean()

		// Get Schedule from autoscaling
		schedule := Schedule{}
		err = dependencies.JSONClient.Do("GET", scheduleURL, nil, &schedule)
		if err != nil {
			return fmt.Errorf("autoscaling API: %s", err)
		}

		sf := SaveFile{
			Rules: rules,
			Schedule: schedule,
		}

		sf.save(dependencies.FileName)
	}

	if dependencies.Method == "import" {
		sf := SaveFile{}
		sf.load(dependencies.FileName)

		err = dependencies.JSONClient.Do("PUT", fullURL, sf.Rules, nil)
		if err != nil {
			return fmt.Errorf("couldn't save rules: %s", err)
		}

		for _, r := range sf.Schedule.Resources {
			err = dependencies.JSONClient.Do("POST", scheduleURL, r, nil)
			if err != nil {
				return fmt.Errorf("couldn't save schedule: %s", err)
			}
		}

	}

	fmt.Println("done.")
	return nil

}

// Run must be implemented by any plugin because it is part of the
// plugin interface defined by the core CLI.
//
// Run(....) is the entry point when the core CLI is invoking a command defined
// by a plugin. The first parameter, plugin.CliConnection, is a struct that can
// be used to invoke cli commands. The second paramter, args, is a slice of
// strings. args[0] will be the name of the command, and will be followed by
// any additional arguments a cli user typed in.
//
// Any error handling should be handled with the plugin itself (this means printing
// user facing errors). The CLI will exit 0 if the plugin exits 0 and will exit
// 1 should the plugin exits nonzero.

func (p *Plugin) Run(cliConnection plugin.CliConnection, args []string) {
	// only handle if actually invoked, else it can't be uninstalled cleanly
	if args[0] != "copy-autoscale" {
		return
	}


	dependencies, err := p.FetchCLIDependencies(cliConnection, args[1:])
	fatalIf(err)

	//jsonData, err := json.MarshalIndent(dependencies.Service, "", "    ")
	//fmt.Println(jsonData)

	if err := p.RunWithError(dependencies); err != nil {
		fmt.Printf("%s", err)
	}
}

// GetMetadata must be implemented as part of the plugin interface
// defined by the core CLI.
//
// GetMetadata() returns a PluginMetadata struct. The first field, Name,
// determines the name of the plugin which should generally be without spaces.
// If there are spaces in the name a user will need to properly quote the name
// during uninstall otherwise the name will be treated as seperate arguments.
// The second value is a slice of Command structs. Our slice only contains one
// Command Struct, but could contain any number of them. The first field Name
// defines the command `cf basic-plugin-command` once installed into the CLI. The
// second field, HelpText, is used by the core CLI to display help information
// to the user in the core commands `cf help`, `cf`, or `cf -h`.
func (p *Plugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "copy-autoscale",
		Version: plugin.VersionType{
			Major: 0,
			Minor: 0,
			Build: 1,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "copy-autoscale",
				HelpText: "Plugin to copy the autoscale settings",

				// UsageDetails is optional
				// It is used to show help of usage of each command
				UsageDetails: plugin.Usage{
					Usage: "$ cf copy-autoscaler helloworld > autoscaler-settings.json\n$ cf copy-autoscaler helloworld autoscaler-settings.json",
				},
			},
		},
	}
}
