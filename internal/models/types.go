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
