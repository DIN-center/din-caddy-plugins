FROM caddy:2.7.6-builder AS builder

# run xcaddy build once to go get common packages
# this will be cached and hopefully make later runs of xcaddy build for our plugin faster
RUN xcaddy build

RUN mkdir /din-plugins
RUN mkdir /din-plugins/lib
RUN mkdir /din-plugins/modules

COPY *.go go.* /din-plugins/
COPY lib/ /din-plugins/lib/
COPY modules/ /din-plugins/modules/

RUN xcaddy build --with github.com/DIN-center/din-caddy-plugins=/din-plugins

FROM caddy:2.7.6

COPY --from=builder /usr/bin/caddy /usr/bin/caddy