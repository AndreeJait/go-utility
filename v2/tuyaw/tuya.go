package tuyaw

import "context"

// Token represents a Tuya access token and its associated metadata.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpireTime   int    `json:"expire_time"` // seconds
	UID          string `json:"uid"`
}

// Device represents a Tuya smart home device.
type Device struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	ProductID    string `json:"product_id"`
	ProductName  string `json:"product_name"`
	ProductImg   string `json:"product_img"`
	Model        string `json:"model"`
	Sub          bool   `json:"sub"`
	UUID         string `json:"uuid"`
	Online       bool   `json:"online"`
	Icon         string `json:"icon"`
	TimeZoneID   string `json:"time_zone_id"`
	LocalKey     string `json:"local_key"`
	ActiveTime   int64  `json:"active_time"`
	CreateTime   int64  `json:"create_time"`
	UpdateStatus int    `json:"update_status"`
	Status       []DeviceStatusPoint `json:"status,omitempty"`
}

// DeviceFunction represents a controllable function point of a device.
type DeviceFunction struct {
	Code   string `json:"code"`
	Type   string `json:"type"`   // Boolean, Integer, Enum, Json
	Values string `json:"values"` // value range description
	Name   string `json:"name"`
	Desc   string `json:"desc"`
}

// DeviceFunctionsResult wraps the functions response from the Tuya API.
// The API returns {"category": "...", "functions": [...]} rather than a plain array.
type DeviceFunctionsResult struct {
	Category  string           `json:"category"`
	Functions []DeviceFunction `json:"functions"`
}

// DeviceSpecification represents the specification of a device (functions + status set).
type DeviceSpecification struct {
	Functions []DeviceFunction    `json:"functions"`
	Status   []DeviceStatusPoint  `json:"status"`
}

// DeviceStatusPoint represents a single status point of a device.
type DeviceStatusPoint struct {
	Code  string `json:"code"`
	Type  string `json:"type"`
	Value any    `json:"value"`
	Name  string `json:"name"`
}

// DeviceCommand represents a command to send to a device.
type DeviceCommand struct {
	Code  string `json:"code"`
	Value any    `json:"value"`
}

// DeviceLog represents a single device log entry.
type DeviceLog struct {
	Time     string `json:"time"`
	Value    string `json:"value"`
	Category string `json:"category"`
}

// DeviceFactoryInfo represents factory information for a device.
type DeviceFactoryInfo struct {
	ID  string `json:"id"`
	MAC string `json:"mac,omitempty"`
	UUID string `json:"uuid,omitempty"`
	SN  string `json:"sn,omitempty"`
}

// SubDevice represents a sub-device under a gateway device.
type SubDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Online   bool   `json:"online"`
	Category string `json:"category"`
	Icon     string `json:"icon"`
}

// ListDevicesOptions provides optional parameters for listing devices.
type ListDevicesOptions struct {
	UID        string // filter by user ID (required for /v1.0/users/{uid}/devices)
	ProductID  string // filter by product ID
	DeviceIDs  []string // filter by device IDs
	PageNo     int    // page number (1-based)
	PageSize   int    // items per page
}

// DeviceLogOptions provides optional parameters for querying device logs.
type DeviceLogOptions struct {
	StartTime int64  // unix milliseconds
	EndTime   int64  // unix milliseconds
	Category  string // log category
	PageNo    int
	PageSize  int
}

// Tuya defines the contract for interacting with the Tuya IoT Cloud API v1.
type Tuya interface {
	// --- Token Management ---

	// GetToken obtains a new access token using simple mode (grant_type=1).
	GetToken(ctx context.Context) (*Token, error)

	// RefreshToken refreshes an access token using a refresh token.
	RefreshToken(ctx context.Context, refreshToken string) (*Token, error)

	// --- Device Management ---

	// GetDevice returns details for a single device.
	GetDevice(ctx context.Context, deviceID string) (*Device, error)

	// ListDevices returns devices based on the provided options.
	// If opts.UID is set, uses /v1.0/users/{uid}/devices.
	ListDevices(ctx context.Context, opts *ListDevicesOptions) ([]Device, error)

	// UpdateDeviceName modifies the name of a device.
	UpdateDeviceName(ctx context.Context, deviceID, name string) error

	// DeleteDevice removes a device.
	DeleteDevice(ctx context.Context, deviceID string) error

	// GetDeviceSpecifications returns the specification (functions + status) of a device.
	GetDeviceSpecifications(ctx context.Context, deviceID string) (*DeviceSpecification, error)

	// GetDeviceFactoryInfos returns factory information for the specified devices.
	GetDeviceFactoryInfos(ctx context.Context, deviceIDs []string) ([]DeviceFactoryInfo, error)

	// ListSubDevices returns sub-devices under a gateway device.
	ListSubDevices(ctx context.Context, deviceID string) ([]SubDevice, error)

	// GetDeviceLogs returns logs for a device.
	GetDeviceLogs(ctx context.Context, deviceID string, opts *DeviceLogOptions) ([]DeviceLog, error)

	// --- Device Control ---

	// SendCommands sends commands to a device.
	SendCommands(ctx context.Context, deviceID string, commands []DeviceCommand) error

	// GetDeviceStatus returns the current status of a device.
	GetDeviceStatus(ctx context.Context, deviceID string) ([]DeviceStatusPoint, error)

	// GetDeviceFunctions returns the instruction set supported by a device.
	GetDeviceFunctions(ctx context.Context, deviceID string) (*DeviceFunctionsResult, error)

	// GetCategoryFunctions returns the instruction set for a product category.
	GetCategoryFunctions(ctx context.Context, category string) (*DeviceFunctionsResult, error)
}