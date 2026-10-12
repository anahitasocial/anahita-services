#!/bin/sh
# Signs somebody in to the lab's GoToSocial and prints a token of theirs.
#   token.sh <email> <password>
G=https://gts.test; J=/tmp/jar; O="urn:ietf:wg:oauth:2.0:oob"
c() { curl -s --cacert /certs/ca.crt -b $J -c $J "$@"; }
app=$(c -X POST $G/api/v1/apps -d client_name=lab -d "redirect_uris=$O" -d "scopes=read write")
id=$(echo "$app" | sed -n 's/.*"client_id":"\([^"]*\)".*/\1/p'); secret=$(echo "$app" | sed -n 's/.*"client_secret":"\([^"]*\)".*/\1/p')
q="client_id=$id&redirect_uri=$O&response_type=code&scope=read+write"
c -o /dev/null "$G/oauth/authorize?$q"
c -o /dev/null -X POST $G/auth/sign_in --data-urlencode "username=$1" --data-urlencode "password=$2"
c -o /dev/null "$G/oauth/authorize?$q"
code=$(c -o /dev/null -w "%{redirect_url}" -X POST "$G/oauth/authorize" | sed -n 's/.*code=\([A-Za-z0-9_-]*\).*/\1/p')
c -X POST $G/oauth/token -d client_id=$id -d client_secret=$secret -d "redirect_uri=$O" -d grant_type=authorization_code -d code=$code | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
