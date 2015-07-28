FROM debian:latest
WORKDIR /home/weave

RUN apt-get update && apt-get install -y curl bzip2

# Install nodejs
RUN curl -sL https://deb.nodesource.com/setup | bash -
RUN apt-get install -y nodejs

# install app and build dependencies
ADD src /home/weave/src/
ADD build /home/weave/build/
ADD package.json webpack.production.config.js server.js /home/weave/
RUN npm install --no-optional; npm run build

# run app server
EXPOSE 8080
ENV NODE_ENV production
CMD ["node", "server.js"]
