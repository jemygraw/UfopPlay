FROM ubuntu:16.04

MAINTAINER wubingheng@qiniu.com

#move files
RUN mkdir -p /root/ossimg/
ADD * /root/ossimg/

#set env variables
EXPOSE 9100

#start service
WORKDIR /root/ossimg
ENTRYPOINT ./qufop qufop.conf
