CREATE TABLE spiks
(
	id BIGINT NOT NULL AUTO_INCREMENT,
	content VARCHAR(140),
	created_at DATETIME,
	PRIMARY KEY(id)
);

CREATE TABLE hashtags
(
	id BIGINT NOT NULL AUTO_INCREMENT,
	name VARCHAR(140),
	created_at DATETIME,
	PRIMARY KEY(id, name)
);

CREATE TABLE spik_hashtags
(
	spik_id BIGINT NOT NULL,
	hashtag_id BIGINT NOT NULL,
	created_at DATETIME,
	PRIMARY KEY(spik_id, hashtag_id)
);

CREATE TABLE user_follow_hashtags
(
	user_id VARCHAR(30) NOT NULL,
	hashtag_id BIGINT NOT NULL,
	created_at DATETIME,
	PRIMARY KEY(user_id, hashtag_id)
);
