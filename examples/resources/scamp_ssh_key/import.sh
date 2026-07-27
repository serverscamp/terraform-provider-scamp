# SSH keys are the one resource with a numeric id.
# The private key is never returned again, so an imported key has none.
terraform import scamp_ssh_key.main 42
