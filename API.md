# root command

invokes the main command

flags:
* 

## service

choose one of the supported services and type it as a command

positional arguments:
* `file`: a list of local path(s) to file(s) to upload

flags:
* `--max-downloads <int>`: self-explanatory
* `--max-days <int>`: number in days after which the file will be deleted on the server
* `--host <url>`: specify a different host than the original (for example, a self-hosted service)

### delete
positional arguments:
* `<delete_url>`: the delete url is ideally given by the service at the time of uploading
