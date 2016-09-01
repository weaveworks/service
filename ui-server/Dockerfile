FROM nginx
MAINTAINER Weaveworks Inc <help@weave.works>
# Need try_files directive for single page app with browser history
COPY default.conf /etc/nginx/conf.d/
COPY client/build /usr/share/nginx/html/
