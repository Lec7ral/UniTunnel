//go:build !windows

package network

type NetworkModeType string

const (
	ModeUCICable   NetworkModeType = "uci_cable"
	ModeWiFiMobile NetworkModeType = "wifi_mobile"
	ModeDirect     NetworkModeType = "direct"
)

type InterfaceStatus struct {
	Alias          string `json:"alias"`
	InterfaceIndex int    `json:"if_index"`
	Metric         int    `json:"metric"`
	AutoMetric     bool   `json:"auto_metric"`
	Status         string `json:"status"`
	MediaType      string `json:"media_type"`
}

func ApplyNetworkMode(mode string) error {
	return nil
}

func GetInterfaces() ([]InterfaceStatus, error) {
	return []InterfaceStatus{
		{
			Alias:      "Simulated Ethernet",
			Metric:     10,
			AutoMetric: false,
			MediaType:  "ethernet",
			Status:     "Connected",
		},
		{
			Alias:      "Simulated Wi-Fi",
			Metric:     100,
			AutoMetric: false,
			MediaType:  "wifi",
			Status:     "Connected",
		},
	}, nil
}
