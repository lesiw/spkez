# spkez

A simple secret store with a casual atmosphere.

## Install

Requires `git`.

### go install

```sh
go install lesiw.io/spkez@latest
```

### curl

```sh
curl lesiw.io/spkez | sh
```

## Usage

``` 
spkez - simple secret manager

  spkez ls                 List keys.
  spkez get [key]          Get value of [key].
  spkez del [key]          Delete [key].
  spkez set [key] [value]  Set [key] to [value].

environment variables

  SPKEZREPO  URL of the git secret store.
  SPKEZPASS  Password for encrypting and decrypting secrets.
```

## Example

```sh
export SPKEZREPO=github.com/example/example.git
# scp-style urls are also supported:
#   export SPKEZREPO=git@github.com:example/example.git
export SPKEZPASS=supersecretpassword
spkez set some/key "secret" # => set "some/key"
spkez get some/key          # => secret
spkez ls                    # => some/key
spkez del some/key          # => deleted "some/key"
```
