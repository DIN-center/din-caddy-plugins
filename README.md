"DIN" reverse proxy 
===============================================

This repo is a _proof of concept_ for a reverse proxy that to din
providers

### xcaddy Development

The easiest way to run the plugin is the use `xcaddy`:

```bash
$ go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
```

By default this will use the `Caddyfile` in the local directory, pass
`--config /path/to/Caddyfile` to use a different one. See the full `xcaddy`
docs for details.

Caddy plugin development is covered via the guide at:
https://caddyserver.com/docs/extending-caddy

**Running locally**

Rename development Caddyfile which contains a single infura test provider
```bash
$ mv Caddyfile.dev Caddyfile
```

Start caddy server at port 8080
```bash
$ xcaddy run
....
2024/12/26 22:26:02.135	INFO	using adjacent Caddyfile
2024/12/26 22:26:02.144	INFO	adapted config to JSON	{"adapter": "caddyfile"}
2024/12/26 22:26:02.144	WARN	Caddyfile input is not formatted; run 'caddy fmt --overwrite' to fix inconsistencies	{"adapter": "caddyfile", "file": "Caddyfile", "line": 33}
2024/12/26 22:26:02.144	INFO	redirected default logger	{"from": "stderr", "to": ".../DIN-center/din-caddy-plugins/caddy.log"}
```

Test the proxy with the following curl command
```bash
$  curl -X POST -H "Content-Type: application/json" -H "Host: ingress.din.dev" \
        --data '{"jsonrpc": "2.0","method": "eth_getBlockByNumber","params": ["latest",false],"id": 5}' \
        http://localhost:8080/eth
```
