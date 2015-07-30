FROM node:0.12-onbuild

# build JS bundle (postinstall has not been executed at this point)
RUN npm run build

# configure port
EXPOSE 8080

# onbuild runs `npm start` serving app from /usr/src/app/build/
