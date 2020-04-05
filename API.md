# common

common commands for all backends

positinal arguments:
* `backend <backend>`: pick one of the supported backends

flags:
* `--host <url>`: specify a different host than the original (for example, a self-hosted service)

## send

positinal arguments:
* `file`: a list of local path(s) to file(s) to upload
* 

flags:
* `--encrypt [key]`: forget this flag for now
* `--max-downloads <int>`: self-explanatory
* `--max-days <int>`: number in days

## get

positional arguments:
* `url`: downloads url

flags:
* `--dir <dir>`: directory to download file

## delete
positional arguments:
* `delete_link`: deletes the link
