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
