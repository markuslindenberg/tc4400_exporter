FROM scratch

ADD tc4400_exporter /tc4400_exporter

ENTRYPOINT ["/tc4400_exporter"]