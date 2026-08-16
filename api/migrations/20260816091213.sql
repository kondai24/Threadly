-- Create "comment_likes" table
CREATE TABLE `comment_likes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` char(36) NOT NULL,
  `comment_id` char(36) NOT NULL,
  `created_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_comment_likes_comment_id` (`comment_id`),
  UNIQUE INDEX `uidx_comment_likes_user_comment` (`user_id`, `comment_id`),
  CONSTRAINT `fk_comment_likes_comment` FOREIGN KEY (`comment_id`) REFERENCES `comments` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_comment_likes_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "post_likes" table
CREATE TABLE `post_likes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` char(36) NOT NULL,
  `post_id` char(36) NOT NULL,
  `created_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_post_likes_post_id` (`post_id`),
  UNIQUE INDEX `uidx_post_likes_user_post` (`user_id`, `post_id`),
  CONSTRAINT `fk_post_likes_post` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_post_likes_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
