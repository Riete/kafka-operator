FROM riet/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/kafka-operator \
    USER_UID=1001 \
    USER_NAME=kafka-operator

# install operator binary
COPY build/_output/bin/kafka-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
