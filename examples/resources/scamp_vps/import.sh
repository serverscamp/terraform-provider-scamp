# The id is the VPS UUID, from the panel, scli vps ls or the API.
# Write the config as the server is before the first apply: a server with backups needs backups = true,
# otherwise the apply turns them off and deletes every backup.
# password and ssh_key_ids cannot be read back; set them in the config and they are only recorded.
terraform import scamp_vps.web 12d46f8c-d66e-4163-99aa-1245d5b06e5b
