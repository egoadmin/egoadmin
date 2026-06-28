#!/bin/sh
sed -i  "s|<body>|<body baseurl=\"$BASE_URL\" asseturl=\"$ASSET_URL\" hostname=\"$HOSTNAME\" env=\"$ENV\">|"  /usr/share/nginx/html/index.html
nginx -g "daemon off;"