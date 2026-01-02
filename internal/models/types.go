package models

// SSHKey represents an SSH key from the API.
type SSHKey struct {
	ID            int    `json:"id"`
	KeyName       string `json:"key_name"`
	KeyType       string `json:"key_type"`
	PublicKey     string `json:"public_key"`
	PrivateKey    string `json:"private_key,omitempty"` // Only returned on generate
	Fingerprint   string `json:"fingerprint"`
	HasPrivateKey bool   `json:"has_private_key,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// SSHKeysListResponse represents GET /ssh-keys response.
type SSHKeysListResponse struct {
	Items []SSHKey `json:"items"`
	Total int      `json:"total"`
}

// DeleteResponse represents DELETE response.
type DeleteResponse struct {
	Message string `json:"message"`
}

// Network represents a network from the API.
type Network struct {
	NetworkUUID string  `json:"network_uuid"`
	Name        string  `json:"name"`
	CIDR        string  `json:"cidr"`
	NetworkType string  `json:"network_type,omitempty"` // "private" or "public"
	RouterUUID  *string `json:"router_uuid"`            // null if not attached
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at,omitempty"`
}

// NetworksListResponse represents GET /networks response.
type NetworksListResponse struct {
	Items []Network `json:"items"`
	Total int       `json:"total"`
}

// NetworkAttachResponse represents POST /networks/{uuid}/attach response.
type NetworkAttachResponse struct {
	NetworkUUID string `json:"network_uuid"`
	RouterUUID  string `json:"router_uuid"`
	Status      string `json:"status"`
}

// Router represents a router from the API.
type Router struct {
	RouterUUID  string `json:"router_uuid"`
	Name        string `json:"name"`
	IPv4Address string `json:"ipv4_address"`
	IPv6Address string `json:"ipv6_address"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// RoutersListResponse represents GET /routers response.
type RoutersListResponse struct {
	Items []Router `json:"items"`
	Total int      `json:"total"`
}

// VMClass represents a VM class from the API.
type VMClass struct {
	ID                     int    `json:"id"`
	Name                   string `json:"name"`
	Description            string `json:"description"`
	CPUCores               int    `json:"cpu_cores"`
	CPUMinUsage            int    `json:"cpu_min_usage"`
	CPUMaxUsage            int    `json:"cpu_max_usage"`
	CheckPeriodMinutes     int    `json:"check_period_minutes"`
	MemoryMB               int    `json:"memory_mb"`
	PricePerHourMillicents float64 `json:"price_per_hour_millicents"`
	IsActive               bool    `json:"is_active"`
	CreatedAt              string  `json:"created_at,omitempty"`
	UpdatedAt              string  `json:"updated_at,omitempty"`
}

// VMClassesListResponse represents GET /vm-classes response.
type VMClassesListResponse struct {
	Items []VMClass `json:"items"`
	Total int       `json:"total"`
}

// StorageClass represents a storage class from the API.
type StorageClass struct {
	ID                       int    `json:"id"`
	Name                     string `json:"name"`
	Description              string `json:"description"`
	MaxSizeGB                int    `json:"max_size_gb"`
	ReadIOPSLimit            int    `json:"read_iops_limit"`
	WriteIOPSLimit           int    `json:"write_iops_limit"`
	ReadBandwidthLimit       int    `json:"read_bandwidth_limit"`
	WriteBandwidthLimit      int    `json:"write_bandwidth_limit"`
	SDSPoolName              string `json:"sds_pool_name"`
	ReplicaCount             int     `json:"replica_count"`
	PricePerGBHourMillicents float64 `json:"price_per_gb_hour_millicents"`
	IsActive                 bool    `json:"is_active"`
	CreatedAt                string  `json:"created_at,omitempty"`
	UpdatedAt                string  `json:"updated_at,omitempty"`
}

// StorageClassesListResponse represents GET /storage-classes response.
type StorageClassesListResponse struct {
	Items []StorageClass `json:"items"`
	Total int            `json:"total"`
}

// NetworkClass represents a network class from the API.
type NetworkClass struct {
	ID                          int    `json:"id"`
	Name                        string `json:"name"`
	Description                 string `json:"description"`
	DownloadMbitLimit           int    `json:"download_mbit_limit"`
	UploadMbitLimit             int    `json:"upload_mbit_limit"`
	IncludedTrafficGB           int     `json:"included_traffic_gb"`
	PricePerHourMillicents      float64 `json:"price_per_hour_millicents"`
	TrafficPricePerGBMillicents float64 `json:"traffic_price_per_gb_millicents"`
	IsActive                    bool    `json:"is_active"`
	CreatedAt                   string  `json:"created_at,omitempty"`
	UpdatedAt                   string  `json:"updated_at,omitempty"`
}

// NetworkClassesListResponse represents GET /network-classes response.
type NetworkClassesListResponse struct {
	Items []NetworkClass `json:"items"`
	Total int            `json:"total"`
}

// VMTemplate represents a VM template from the API.
type VMTemplate struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	APIName   string `json:"api_name"`
	OSFamily  string `json:"os_family"`
	OSType    string `json:"os_type"`
	Version   string `json:"version"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// VMTemplatesListResponse represents GET /vm-templates response.
type VMTemplatesListResponse struct {
	Items []VMTemplate `json:"items"`
	Total int          `json:"total"`
}

// VMNetworkInfo represents network info of a VM.
type VMNetworkInfo struct {
	IPInternal  string `json:"ip_internal"`
	MACAddress  string `json:"mac_address"`
	InterfaceID string `json:"interface_id"`
	IPv6Address string `json:"ipv6_address"`
	IPv6Gateway string `json:"ipv6_gateway"`
	PublicIPv4  string `json:"public_ip_v4"`
	PublicIPv6  string `json:"public_ip_v6"`
}

// VMLimits represents resource limits of a VM.
type VMLimits struct {
	RootDiskReadIOPSLimit        int `json:"root_disk_read_iops_limit"`
	RootDiskWriteIOPSLimit       int `json:"root_disk_write_iops_limit"`
	RootDiskReadBandwidthLimit   int `json:"root_disk_read_bandwidth_limit"`
	RootDiskWriteBandwidthLimit  int `json:"root_disk_write_bandwidth_limit"`
	PublicIfaceDownloadMbitLimit int `json:"public_iface_download_mbit_limit"`
	PublicIfaceUploadMbitLimit   int `json:"public_iface_upload_mbit_limit"`
}

// VM represents a virtual machine from the API.
type VM struct {
	ID             int            `json:"id"`
	VMUUID         string         `json:"vm_uuid"`
	VMName         string         `json:"vm_name"`
	DisplayName    string         `json:"display_name"`
	CPUCores       int            `json:"cpu_cores"`
	CPUSockets     int            `json:"cpu_sockets"`
	CPUMaxUsage    int            `json:"cpu_max_usage"`
	MemoryMB       int            `json:"memory_mb"`
	DiskGB         int            `json:"disk_gb"`
	VMClassID      int            `json:"vm_class_id"`
	StorageClassID int            `json:"storage_class_id"`
	NetworkClassID int            `json:"network_class_id"`
	VMTemplateID   int            `json:"vm_template_id"`
	NetworkUUID    string         `json:"network_uuid"`
	SSHKeyID       *int           `json:"ssh_key_id"`
	Network        *VMNetworkInfo `json:"network"`
	Limits         *VMLimits      `json:"limits"`
	OSUser         string         `json:"os_user"`
	OSPassword     string         `json:"os_password"`
	Status         string         `json:"status"`
	State          string         `json:"state"`
	CreatedAt      string         `json:"created_at,omitempty"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
}

// VMsListResponse represents GET /vms response.
type VMsListResponse struct {
	Items []VM `json:"items"`
	Total int  `json:"total"`
}

// VMCreateResponse represents POST /vms response.
type VMCreateResponse struct {
	ID          int    `json:"id"`
	VMUUID      string `json:"vm_uuid"`
	VMName      string `json:"vm_name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	OSUser      string `json:"os_user"`
	OSPassword  string `json:"os_password"`
	IPInternal  string `json:"ip_internal"`
	IPv6Address string `json:"ipv6_address"`
	PublicIPv4  string `json:"public_ip_v4"`
	PublicIPv6  string `json:"public_ip_v6"`
}

// VolumeLimits represents I/O limits of a volume.
type VolumeLimits struct {
	ReadIOPSLimit       int `json:"read_iops_limit"`
	WriteIOPSLimit      int `json:"write_iops_limit"`
	ReadBandwidthLimit  int `json:"read_bandwidth_limit"`
	WriteBandwidthLimit int `json:"write_bandwidth_limit"`
}

// Volume represents a volume from the API.
type Volume struct {
	ID             int           `json:"id"`
	DiskUUID       string        `json:"disk_uuid"`
	DisplayName    string        `json:"display_name"`
	SizeGB         int           `json:"size_gb"`
	StorageClassID int           `json:"storage_class_id"`
	Limits         *VolumeLimits `json:"limits"`
	SDSPoolName    string        `json:"sds_pool_name"`
	VMUUID         *string       `json:"vm_uuid"` // null if not attached
	State          string        `json:"state"`
	CreatedAt      string        `json:"created_at,omitempty"`
	UpdatedAt      string        `json:"updated_at,omitempty"`
}

// VolumesListResponse represents GET /volumes response.
type VolumesListResponse struct {
	Items []Volume `json:"items"`
	Total int      `json:"total"`
}

// VolumeCreateResponse represents POST /volumes response.
type VolumeCreateResponse struct {
	ID          int    `json:"id"`
	DiskUUID    string `json:"disk_uuid"`
	DisplayName string `json:"display_name"`
	SizeGB      int    `json:"size_gb"`
	Status      string `json:"status"`
}

// VolumeAttachResponse represents POST /volumes/{uuid}/attach response.
type VolumeAttachResponse struct {
	DiskUUID string `json:"disk_uuid"`
	VMUUID   string `json:"vm_uuid"`
	Status   string `json:"status"`
}
