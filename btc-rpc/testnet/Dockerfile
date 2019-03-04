FROM debian:stretch-slim

ENV HOME /bitcoin

ENV BITCOIN_VERSION 0.17.1

RUN groupadd -g 1000 bitcoin && useradd -u 1000 -g bitcoin -s /bin/bash -m -d /bitcoin bitcoin

RUN apt-get update -y \
  && apt-get install -y curl unzip procps \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV BITCOIN_SHASUM="53ffca45809127c9ba33ce0080558634101ec49de5224b2998c489b6d0fc2b17  bitcoin-${BITCOIN_VERSION}-x86_64-linux-gnu.tar.gz"

RUN curl -SLO https://bitcoin.org/bin/bitcoin-core-${BITCOIN_VERSION}/bitcoin-${BITCOIN_VERSION}-x86_64-linux-gnu.tar.gz \
  && echo "${BITCOIN_SHASUM}" | sha256sum -c \
  && tar -xzf *.tar.gz -C /opt \
  && rm *.tar.gz

ENV PATH=/opt/bitcoin-${BITCOIN_VERSION}/bin:$PATH

EXPOSE 8332 8334

WORKDIR /bitcoin

ADD wallet.dat /bitcoin

COPY btc_init.sh /btc_init.sh

CMD ["/btc_init.sh"]