FROM node:0.10
WORKDIR /home/weave
COPY webpack.local.config.js webpack.production.config.js .eslintignore .eslintrc /home/weave/
COPY package.json /home/weave/
RUN npm install
