package com.brotherlogic.recordcollection.storage;

import com.brotherlogic.recordcollection.DiscogsToken;
import com.brotherlogic.recordcollection.RequestBuilder;
import com.brotherlogic.recordcollection.ScribeRetriever;

import java.util.Map;

import org.scribe.model.Token;

public interface Storage {
  Token getToken(String userKey);
  void storeToken(Token token);
}
