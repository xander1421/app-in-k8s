FROM ruby:3.1
RUN apt-get update -yqq && apt-get install -yqq --no-install-recommends \
        supervisor \
    && apt-get -q clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /usr/src/app

COPY Gemfile* ./
RUN bundle install

ADD supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY . .

EXPOSE 3000

CMD ["supervisord"]