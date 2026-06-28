.PHONY: db.mysql
db.mysql:
	@mysql -h$(MYSQL_HOST) -P$(MYSQL_PORT) -u$(MYSQL_USER) -p$(MYSQL_PASSWORD) -c