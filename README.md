# sendall

Upload your files to popular one-off file-sharing servies through one interface. You can self-host most of these services

## Usage

Upload a file to service transfer.sh with a download limit of 7 downloads
```
sendall transfer <file> --downloads 7
```

Delete your just uploaded file from the transfer.sh server
```
sendall transfer delete <exact_url_you_received_from_the_server>
```

Upload a markdown document to your self-hosted private bin instance, with an expiration time of 10 minutes
```
sendall privatebin <file> --host myhost.tld --format markdown --days 10min
```

## Supported Services
* transfer.sh
* private bin 

### Notes
* The server at [transfer.sh](https://transfer.sh) is not updated with the latest code from the original repository. The APIs are thus not compatible

## TODO

* Add support the following services:

    [ ] [Firefox send](https://github.com/mozilla/send) (maybe we can use this [rust client](https://github.com/timvisee/ffsend) ? It has an [api](https://github.com/timvisee/ffsend-api) component too. Also, there are two python client implementation [here](https://github.com/nneonneo/ffsend) and [here](https://github.com/ehuggett/send-cli)

    [ ] WeTransfer
