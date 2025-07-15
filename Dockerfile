FROM golang:1.21-alpine AS builder

# Instala mysql-client (mariadb-client no caso de MariaDB)
RUN apk add --no-cache mariadb-client

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . . 
RUN go build -o mysql-backup-system main.go

FROM alpine:latest

# Instala mysql-client (mariadb-client), sshd e ca-certificates
RUN apk add --no-cache mariadb-client ca-certificates openssh

# Cria pastas
WORKDIR /app
RUN mkdir -p /app/backups /app/logs /var/run/sshd /root/.ssh

# Copia binário
COPY --from=builder /app/mysql-backup-system . 

# Configura SSH
RUN echo "root:root" | chpasswd && \
    ssh-keygen -A && \
    sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#Port 22/Port 22/' /etc/ssh/sshd_config && \
    echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config

# Expõe portas
EXPOSE 8030 22

# Inicia SSH + aplicação
CMD /usr/sbin/sshd && ./mysql-backup-system version: '3.8'
