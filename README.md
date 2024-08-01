# 1cloud.ru dns challenge with certbot manual-auth-hook


## Example 

### add.sh
  ```shell
#!/bin/sh

/scripts/cloud_dns_challenge -api "api_secret_key"

sleep 45
```

### del.sh
  ```shell
#!/bin/sh

/scripts/cloud_dns_challenge -del -api "api_secret_key"
```

### docker 
```shell
#!/bin/sh

docker pull certbot/certbot:amd64-latest

docker run -it --rm --name certbot \
    --volume "/mnt/user/appdata/certBot/letsencrypt:/etc/letsencrypt" \
    --volume "/mnt/user/appdata/certBot/lib-letsencrypt:/var/lib/letsencrypt" \
    --volume "/mnt/user/appdata/certBot/scripts:/scripts" \
    --volume "/mnt/user/appdata/certBot/log:/var/log/letsencrypt" \
    certbot/certbot:amd64-latest certonly \
    --preferred-challenges=dns \
    --server https://acme-v02.api.letsencrypt.org/directory  \
    --manual --manual-public-ip-logging-ok \
    --manual-auth-hook /scripts/add.sh \
    --manual-cleanup-hook /scripts/del.sh \
    --email example@mail.ru --no-eff-email --agree-tos \
    --domain example.ru --domain *.example.ru 
```
