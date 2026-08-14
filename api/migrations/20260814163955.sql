-- Create "comments" table
CREATE TABLE `comments` (
  `id` char(36) NOT NULL,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `post_id` char(36) NOT NULL,
  `author_id` char(36) NOT NULL,
  `parent_id` char(36) NULL,
  `content` text NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_comments_author_id` (`author_id`),
  INDEX `idx_comments_deleted_at` (`deleted_at`),
  INDEX `idx_comments_parent_id` (`parent_id`),
  INDEX `idx_comments_post_id` (`post_id`),
  CONSTRAINT `fk_comments_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_comments_post` FOREIGN KEY (`post_id`) REFERENCES `posts` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_comments_replies` FOREIGN KEY (`parent_id`) REFERENCES `comments` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
