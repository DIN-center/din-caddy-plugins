DIN's sign-in-with-ethereum authentication protocol requires cryptographic signing, making it difficult to execute from the command line via tools like CURL. This is a simple command-line tool that is designed to streamline this process.

# Usage

There are several ways to use the `siwe-token` utility. All of them require you to have a hex encoded Ethereum private key in a file, the address of which must be in the whitelist of the DIN provider you are trying to connect to. Given a private key in a file called "secret", you can run `siwe-token` with:

```
./siwe-token https://din.rivet.cloud/auth secret
```

And get an output that looks something like:

```
Your signing address: 0x26a52588627FFF0e0EC66f070ee49ecBFECf3BC2
Add to CURL:
-H 'x-api-key: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjY2MDYyMjksImlhdCI6MTcyNjYwMjYyOX0.ITAs5U4-2BMqLR0mUFdkDVAfYj4iOnfRpqOvwEG-cIw'
```

If you didn't know the address of your key and need to provide it to an operator for whitelisting, you can give them the signing address listed here. From there, you can copy the last line and add it to a curl command, eg:

```
curl https://din.rivet.cloud/eth --data '{"jsonrpc": "2.0", "id": 0, "method": "eth_blockNumber"}' -H 'content-type: application/json' -H 'x-api-key: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjY2MDYyMjksImlhdCI6MTcyNjYwMjYyOX0.ITAs5U4-2BMqLR0mUFdkDVAfYj4iOnfRpqOvwEG-cIw'
```

And this request should work. The key returned by `siwe-token` will expire after a short while, so you will need to renew it occasionally.

## Environment Variables

Most of the output from `siwe-token` is on stderr, while the most important parts are on stdout. This means you can set variables to the important part easily.

If you run:

```
SIWE_HEADER=$(./siwe-token https://din.rivet.cloud/auth secret 2>/dev/null)
curl https://din.rivet.cloud/eth --data '{"jsonrpc": "2.0", "id": 0, "method": "eth_blockNumber"}' -H 'content-type: application/json' -H $SIWE_HEADER
```

You can save yourself a little trouble of copying and pasting, and have your commands looking a bit more legible.

For one-offs, you could even do:

```
curl https://din.rivet.cloud/eth --data '{"jsonrpc": "2.0", "id": 0, "method": "eth_blockNumber"}' -H 'content-type: application/json' -H $(./siwe-token https://din.rivet.cloud/auth secret 2>/dev/null)
```

But this isn't recommended for series of requests, as it will have to execute the authentication handshake with every request, adding overhead.

# FAQ

**Q:** How do I get an Ethereum secret key I can use for this?

**A:** On Unix-like systems, you can run:

```
python3 -c 'import string, secrets; print("".join(secrets.choice(string.hexdigits[:-6]) for _ in range(64)))' > secret
```

To generate a strong random key for use with `siwe-token`.