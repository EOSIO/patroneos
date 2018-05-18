FROM golang:stretch as builder

ADD . /repo
RUN cd /repo && go build -o patroneosd *.go

FROM haproxy:1.8

RUN apt update && apt install -y fail2ban iptables && rm -rf /var/lib/apt/lists/* && rm /etc/fail2ban/filter.d/*.conf

RUN mkdir -p /etc/patroneos

# Shell script to start HAProxy, fail2ban and patroneosd
COPY docker/proxy/start.sh /start.sh

# HAProxy
COPY docker/proxy/haproxy/haproxy.cfg /usr/local/etc/haproxy/haproxy.cfg
COPY docker/proxy/haproxy/ssl.pem /usr/local/etc/haproxy/ssl.pem

# patroneos
COPY --from=builder /repo/patroneosd /usr/bin/patroneosd
COPY docker/proxy/patroneos/config.json /etc/patroneos/config.json
RUN touch /var/log/patroneosd.log

# fail2ban
COPY docker/proxy/fail2ban/action.d/*.conf /etc/fail2ban/action.d/
COPY docker/proxy/fail2ban/jail.d/*.conf /etc/fail2ban/jail.d/
COPY docker/proxy/fail2ban/filter.d/*.conf /etc/fail2ban/filter.d/
COPY docker/proxy/fail2ban/fail2ban.d/*.conf /etc/fail2ban/fail2ban.d/
RUN rm /etc/fail2ban/jail.d/defaults-debian.conf && mkdir -p /var/run/fail2ban

EXPOSE 443/tcp
EXPOSE 8080/tcp
EXPOSE 8888/tcp

USER root

CMD ["/start.sh"]
