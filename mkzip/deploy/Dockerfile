FROM ubuntu:14.04

MAINTAINER jinxinxin@qiniu.com

#move files
RUN mkdir -p /root/mkzip/
ADD mkzip/* /root/mkzip/

#set env variables
EXPOSE 9100

#start service
WORKDIR /root/mkzip
ENTRYPOINT ./qufop qufop.conf