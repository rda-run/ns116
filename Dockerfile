FROM almalinux:10
RUN useradd -u 10001 ns116

FROM scratch
ARG VERSION
ARG BUILD_DATE
LABEL org.opencontainers.image.title="NS116 Server"
LABEL org.opencontainers.image.description="NS116 is a web interface for managing DNS records with multi-user support, role-based access control, and audit logging."
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.source="https://github.com/rda-run/ns116"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.vendor="rda.run"
COPY --from=0 /etc/passwd /etc/passwd
COPY --from=0 /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /etc/ssl/certs/ca-certificates.crt
COPY bin/ns116 /bin/ns116
USER ns116
CMD ["/bin/ns116"]
