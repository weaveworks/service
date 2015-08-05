FROM node:0.10
WORKDIR /home/weave
COPY package.json karma.config.js karma.cross-browser.config.js /home/weave/
COPY webpack.local.config.js webpack.production.config.js /home/weave/
RUN npm install
