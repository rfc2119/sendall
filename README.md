# sendall

## Usage

upload a file to backend `ffsend` (firefox send) using default credentials (anonymous)
```
./sendall --backend ffsend <file>
```

## Credentials
`sendall` uses credentials for backends that support it. For example, [Firefox send](https://github.com/mozilla/send) do it

## Backends
### Firefox Send
why implement a new client instead of the well-made [ffsend](https://github.com/timvisee/ffsend) client ? well, it's written in rust, and I am not aware yet how to merge the two binaries (perhaps bundle it as a separate dependency ?). Maybe we can use its the [api](https://github.com/timvisee/ffsend-api) component and build the rest in Go ? I'm not sure yet

There are two python client implementation [here](https://github.com/nneonneo/ffsend) and [here](https://github.com/ehuggett/send-cli)

### transfer.sh

### wetransfer

## TODO
* use the following as a backend
    [] firefox send
    [] transfer.sh
    [] WeTransfer
