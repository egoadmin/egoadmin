FROM nginx:1.21.1
COPY dist /usr/share/nginx/html
COPY ./start.sh /app/
RUN chmod +x /app/start.sh
EXPOSE 80
ENTRYPOINT ["/app/start.sh"]