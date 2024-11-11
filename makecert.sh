#!/bin/bash

openssl genrsa -out server.key 2048
openssl ecparam -genkey -name secp384r1 -out privkey.pem
openssl req -new -x509 -sha256 -key privkey.pem -out fullchain.pem -days 3650