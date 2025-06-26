# copy-trade-bot


部署：

server {
        # Uncomment the next line to enable listening on port 80
        server_name copytrade0.beyondpump.app;
        location / {
           # proxy_set_header   X-Forwarded-For $remote_addr;
           # proxy_set_header   Host $http_host;
            proxy_set_header Connection '';
            proxy_http_version 1.1;
            chunked_transfer_encoding off;
            proxy_pass   http://127.0.0.1:3000;
            proxy_buffering off;
            proxy_cache off;
         }
   }
