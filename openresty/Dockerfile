FROM openresty/openresty:alpine

ARG BACKEND_RESPONSE_TIMEOUT
ENV BACKEND_RESPONSE_TIMEOUT=${BACKEND_RESPONSE_TIMEOUT}
ARG NGX_CORAZA_UPSTREAM
ENV NGX_CORAZA_UPSTREAM=${NGX_CORAZA_UPSTREAM}

RUN apk add --no-cache bash curl gettext

COPY waf /etc/openresty/waf
COPY nginx.conf /tmp/nginx.conf.template
RUN envsubst '$$NGX_CORAZA_UPSTREAM $$BACKEND_RESPONSE_TIMEOUT' < /tmp/nginx.conf.template > /usr/local/openresty/nginx/conf/nginx.conf
