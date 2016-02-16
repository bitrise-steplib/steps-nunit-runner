require 'optparse'
require 'fileutils'
require 'tmpdir'

require_relative 'xamarin-builder/builder'

# -----------------------
# --- Functions
# -----------------------

def fail_with_message(message)
  puts "\e[31m#{message}\e[0m"
  exit(1)
end

def error_with_message(message)
  puts "\e[31m#{message}\e[0m"
end

def to_bool(value)
  return true if value == true || value =~ (/^(true|t|yes|y|1)$/i)
  return false if value == false || value.nil? || value == '' || value =~ (/^(false|f|no|n|0)$/i)
  fail_with_message("Invalid value for Boolean: \"#{value}\"")
end

# -----------------------
# --- Main
# -----------------------

#
# Parse options
options = {
    solution: nil,
    configuration: nil,
    platform: nil
}

parser = OptionParser.new do |opts|
  opts.banner = 'Usage: step.rb [options]'
  opts.on('-s', '--solution path', 'Solution') { |s| options[:solution] = s unless s.to_s == '' }
  opts.on('-c', '--configuration config', 'Configuration') { |c| options[:configuration] = c unless c.to_s == '' }
  opts.on('-l', '--platform platform', 'Platform') { |l| options[:platform] = l unless l.to_s == '' }
  opts.on('-o', '--options options', 'Nunit options') { |o| options[:options] = o unless o.to_s == '' }
  opts.on('-h', '--help', 'Displays Help') do
    exit
  end
end
parser.parse!

#
# Print options
puts
puts '========== Configs =========='
puts " * solution: #{options[:solution]}"
puts " * configuration: #{options[:configuration]}"
puts " * platform: #{options[:platform]}"

#
# Validate options
fail_with_message("No solution file found at path: #{options[:solution]}") unless options[:solution] && File.exist?(options[:solution])
fail_with_message('No configuration environment found') unless options[:configuration]
fail_with_message('No platform environment found') unless options[:platform]

#
# Main
builder = Builder.new(options[:solution], options[:configuration], options[:platform], nil)
begin
  # The solution has to be built before runing the nunit tests
  # builder.build_solution

  # Executing nunit tests
  builder.run_nunit_tests(options[:options])
rescue => ex
  fail_with_message("nunit test failed: #{ex}")
end
