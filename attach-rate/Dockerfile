FROM centos:7

RUN yum update -y && \
    yum clean all && \
    rm -rf /var/cache/yum

COPY filecounts /

RUN chmod +x filecounts

ENTRYPOINT ["/filecounts"]
