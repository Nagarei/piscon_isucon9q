UPDATE `users`
  INNER JOIN `password_table` ON `users`.`hashed_password` = `password_table`.`hashed_password`
  SET    `users`.`hashed_password` = `password_table`.`weak_hashed_password`;

ALTER TABLE `items` ADD `parent_category_id` int unsigned;
UPDATE `items`
  JOIN `categories` ON `items`.`category_id` = `categories`.`id`
  SET    `items`.`parent_category_id` = `categories`.`parent_id`;
ALTER TABLE items ADD INDEX idx_pcid_createdat_id (`parent_category_id`, `created_at`, `id`);

set global slow_query_log = ON;
