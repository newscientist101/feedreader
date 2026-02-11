-- Add huggingface as a feed type
-- The feed_type column already supports 'custom_scraper', we add 'huggingface'
-- Config stored in scraper_config column as JSON

-- Example configs:
-- User models: {"type": "user_models", "identifier": "openai", "limit": 20}
-- Collection: {"type": "collection", "identifier": "open-llm-leaderboard/open-llm-leaderboard-best-models-652d6c7965a4619fb5c27a03"}
-- User posts: {"type": "user_posts", "identifier": "huggingface"}
-- Daily papers: {"type": "daily_papers", "identifier": "", "limit": 30}
