#!/usr/bin/env bash
index_name="combinedpostpublicationevents"
file_path=$(dirname "$0")/referenceSchema.json
pushd $(dirname "$0"); git restore "$file_path"; popd

set_total_fields_limit() {
  file_path=$1
  limit=$2
  #TODO --arg
  #jq < "$file_path"_2 '.settings.index += {mapping:{total_fields:{limit:$limit}}}' > "$file_path"_2
  jq < "$file_path" '.settings.index += {mapping:{total_fields:{limit:12000}}}' > "$file_path"_2
  mv "$file_path"_2 "$file_path"
}

remove_spaces() {
  file_path=$1
  sed -i 's/ //g' "$file_path"
  sed -i -z 's/\n//g' "$file_path"
}

# `_all` field
# Deprecated in 6.0.0.
# `_all` may no longer be enabled for indices created in 6.0+, use a custom field and the mapping copy_to parameter
# The _all field is a special catch-all field which concatenates the values of all of the other fields into one big
# string, using space as a delimiter, which is then analyzed and indexed, but not stored. This means that it can be
# searched, but not retrieved.
# https://www.elastic.co/guide/en/elasticsearch/reference/current/copy-to.html#copy-to
replace_include_in_all() {
  file_path=$1
  jq < "$file_path" '(..|objects|select(has("_all") and has("properties") and (._all.enabled == true)).properties) += {"meta_all_fields":{"type":"text","index":true}}' > "$file_path"_2
  jq < "$file_path"_2 'walk(if type == "object" and has("_all") then del(._all) else . end)' > "$file_path"
  remove_spaces "$file_path"
  sed -i 's/,"include_in_all":true/,"copy_to":"meta_all_fields"/g' "$file_path"
  sed -i 's/,"include_in_all":false//g' "$file_path"
}

# In the release of Elasticsearch 5.0, removal of the string type. The background for this change is that we think the string type is confusing:
# Elasticsearch has two very different ways to search strings. You can either search whole values, that we often refer to as keyword search,
# or individual tokens, that we usually refer to as full-text search. The former strings should be mapped as a not_analyzed string while the latter
# should be mapped as an analyzed string.
# But the fact that the same field type is used for these two very different use-cases is causing problems since some options only make sense for
# one of the use case. For instance, position_increment_gap makes little sense for a not_analyzed string and it is not obvious whether ignore_above
# applies to the whole value or to individual tokens in the case of an analyzed string (in case you wonder: it does apply to the whole value, limits
# on individual tokens can be applied with the limit token filter).
# To avoid these issues, the string field has split into two new types: text, which should be used for full-text search, and keyword, which should
# be used for keyword search.
# https://www.elastic.co/blog/strings-are-dead-long-live-strings
change_type_string() {
  remove_spaces "$file_path"
  sed -i 's/"type":"string","index":"analyzed"/"type":"text","index":true/g' "$file_path"
  sed -i 's/"type":"string","index":"not_analyzed"/"type":"keyword","index":true/g' "$file_path"
  sed -i 's/"type":"string"/"type":"text"/g' "$file_path"
}

move_mappings_fields_to_a_nested_properties_object() {
  file_path=$1
  jq < "$file_path" '.mappings |= { properties: .}' > "$file_path"_2
  mv "$file_path"_2 "$file_path"
}

reformat_json() {
  jq < "$file_path" '.' > "$file_path"_2
  mv "$file_path"_2 "$file_path"
}

set_total_fields_limit "$file_path" 12000
move_mappings_fields_to_a_nested_properties_object "$file_path"
replace_include_in_all "$file_path"
change_type_string "$file_path"
reformat_json "$file_path"
# exit 0
curl -X PUT "http://localhost:9200/$index_name?pretty" -H 'Content-Type: application/json' --data @"$file_path"
curl "http://localhost:9200/$index_name/_mapping?pretty" | jq .
