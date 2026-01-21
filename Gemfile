source 'https://rubygems.org'
git_source(:github) { |repo| "https://github.com/#{repo}.git" }

ruby '>= 3.0.0'

gem 'rails', '~> 7.0.0'
gem 'puma', '~> 6.0'
gem 'pg', '~> 1.5'
gem 'redis', '~> 4.0'
gem 'sneakers'
gem 'json'
gem 'logger'
gem 'bootsnap', '>= 1.4.2', require: false
gem 'searchkick'
gem 'elasticsearch', '~> 7.0'
gem 'sidekiq'
gem 'dotenv-rails', groups: [:development, :test]

group :development, :test do
  gem 'rspec-rails', '~> 3.9'
  gem 'byebug', platforms: [:mri, :mingw, :x64_mingw]
end

group :development do
  gem 'listen', '~> 3.8'
  gem 'spring'
  gem 'spring-watcher-listen', '~> 2.0.0'
end

gem 'tzinfo-data', platforms: [:mingw, :mswin, :x64_mingw, :jruby]
