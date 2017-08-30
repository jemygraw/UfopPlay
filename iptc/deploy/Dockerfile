FROM ubuntu:14.04

MAINTAINER jinxinxin@qiniu.com

RUN apt-get update
RUN apt-get install -y libiptcdata-dev

#move files
RUN mkdir -p /root/iptc/
ADD iptc/* /root/iptc/

#set env variables
EXPOSE 9100

#start service
WORKDIR /root/iptc
ENTRYPOINT ./qufop qufop.conf