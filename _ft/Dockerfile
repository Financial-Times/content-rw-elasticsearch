#For local testing only
FROM peteclarkft/ersatz:0.0.4

# UPP Suggestions Umbrella Mock
COPY ./ersatz-fixtures.yml /_ft/ersatz-fixtures.yml

WORKDIR /
CMD [ "/ersatz", "--fixtures", "/_ft/ersatz-fixtures.yml", "--port", "9000"  ]

