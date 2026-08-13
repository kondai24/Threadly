-- Create "users" table
CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `username` varchar(32) NOT NULL,
  `password_hash` varchar(255) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_users_deleted_at` (`deleted_at`),
  UNIQUE INDEX `idx_users_username` (`username`)
) CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Create "posts" table
CREATE TABLE `posts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `author_id` bigint unsigned NOT NULL,
  `title` longtext NULL,
  `content` longtext NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_posts_deleted_at` (`deleted_at`),
  INDEX `idx_posts_author_id` (`author_id`),
  CONSTRAINT `fk_posts_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
