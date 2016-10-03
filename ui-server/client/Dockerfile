FROM node:4.2.2
WORKDIR /home/weave
COPY package.json /home/weave/
ENV NPM_CONFIG_LOGLEVEL=warn NPM_CONFIG_PROGRESS=false
RUN npm install
COPY . /home/weave
EXPOSE 80 4042 4046 4047 4048
CMD ["npm", "start"]
