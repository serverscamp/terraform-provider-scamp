package models

type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

type Flavor struct {
    ID     *int     `json:"id"`
    Name   string   `json:"name"`
    VCores *int     `json:"vcores"`
    RAM    *float64 `json:"ram"`
    Disk   *float64 `json:"disk"`
    Price  *float64 `json:"price"`
}
type FlavorsData struct {
    Date         string   `json:"date"`
    RequestID    string   `json:"request_id"`
    FromCache    bool     `json:"from_cache"`
    PriceUnit    string   `json:"price_unit"`
    Currency     string   `json:"currency"`
    CurrencyUnit string   `json:"currency_unit"`
    CPUUnit      string   `json:"cpu_unit"`
    RAMUnit      string   `json:"ram_unit"`
    DiskUnit     string   `json:"disk_unit"`
    Count        int      `json:"count"`
    Total        int      `json:"total"`
    Page         int      `json:"page"`
    PerPage      int      `json:"per_page"`
    Flavors      []Flavor `json:"flavors"`
}
type FlavorsResp struct {
    OK    bool        `json:"ok"`
    Data  FlavorsData `json:"data"`
    Error *APIError   `json:"error,omitempty"`
}

type Limit struct {
    ID        *int   `json:"id"`
    Type      string `json:"type"`
    Limit     *int   `json:"limit"`
    Used      *int   `json:"used"`
    Remaining *int   `json:"remaining"`
}
type LimitsData struct {
    Date      string  `json:"date"`
    RequestID string  `json:"request_id"`
    FromCache bool    `json:"from_cache"`
    Count     int     `json:"count"`
    Total     int     `json:"total"`
    Page      int     `json:"page"`
    PerPage   int     `json:"per_page"`
    Limits    []Limit `json:"limits"`
}
type LimitsResp struct {
    OK    bool       `json:"ok"`
    Data  LimitsData `json:"data"`
    Error *APIError  `json:"error,omitempty"`
}

type Image struct {
    ID           *int   `json:"id"`
    Name         string `json:"name"`
    Version      string `json:"version"`
    ShortName    string `json:"short_name"`
    DistroFamily string `json:"distro_family"`
}
type ImagesData struct {
    Date      string  `json:"date"`
    RequestID string  `json:"request_id"`
    FromCache bool    `json:"from_cache"`
    Count     int     `json:"count"`
    Total     int     `json:"total"`
    Page      int     `json:"page"`
    PerPage   int     `json:"per_page"`
    Images    []Image `json:"images"`
}
type ImagesResp struct {
    OK    bool       `json:"ok"`
    Data  ImagesData `json:"data"`
    Error *APIError  `json:"error,omitempty"`
}

type SSHKey struct {
    ID           int    `json:"id"`
    Name         string `json:"name"`
    Fingerprint  string `json:"fingerprint"`
    Protected    bool   `json:"protected"`
    CreatedAt    string `json:"created_at"`
    HasPrivate   bool   `json:"has_private,omitempty"`
    ServersCount int    `json:"servers_count,omitempty"`
}
type SSHKeysListData struct {
    Count   int      `json:"count"`
    Page    int      `json:"page,omitempty"`
    PerPage int      `json:"per_page,omitempty"`
    Keys    []SSHKey `json:"keys"`
}
type SSHKeysListResp struct {
    OK    bool            `json:"ok"`
    Data  SSHKeysListData `json:"data"`
    Error *APIError       `json:"error,omitempty"`
}
type SSHKeyResp struct {
    OK    bool      `json:"ok"`
    Data  SSHKey    `json:"data"`
    Error *APIError `json:"error,omitempty"`
}

type TaskData struct {
    ID         string          `json:"id"`
    Status     string          `json:"status"`
    Progress   int             `json:"progress"`
    Error      bool            `json:"error"`
    ErrorData  string          `json:"error_data"`
    ResourceID any             `json:"resource_id"`
}
type TaskResp struct {
    OK    bool      `json:"ok"`
    Data  TaskData  `json:"data"`
    Error *APIError `json:"error,omitempty"`
}

type InstancesData struct {
    Date      string     `json:"date"`
    RequestID string     `json:"request_id"`
    FromCache bool       `json:"from_cache"`
    Count     int        `json:"count"`
    Total     int        `json:"total"`
    Page      int        `json:"page"`
    PerPage   int        `json:"per_page"`
    Instances []Instance `json:"instances"`
}
type Instance struct {
    ID           *int     `json:"id"`
    Name         string   `json:"name"`
    OS           string   `json:"os"`
    DistroBase   string   `json:"distro_base"`
    IPv4         string   `json:"ipv4"`
    IPv6         string   `json:"ipv6"`
    Running      bool     `json:"running"`
    CreatedAt    string   `json:"created_at"`
    AgeDays      *int     `json:"age_days"`
    PriceMonth   *float64 `json:"price_month"`
    Flavor       string   `json:"flavor"`
    CPUs         *int     `json:"cpus"`
    RAM          *float64 `json:"ram"`
    Disk         *float64 `json:"disk"`
    VMID         string   `json:"vmid"`
    SSHKeyID     *int     `json:"ssh_key_id"`
    FlavorID     *int     `json:"flavor_id"`
    ImageID      *int     `json:"image_id"`
}
type InstancesResp struct {
    OK    bool          `json:"ok"`
    Data  InstancesData `json:"data"`
    Error *APIError     `json:"error,omitempty"`
}
