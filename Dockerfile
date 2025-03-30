FROM ubuntu:latest
LABEL authors="fello"

ENTRYPOINT ["top", "-b"]