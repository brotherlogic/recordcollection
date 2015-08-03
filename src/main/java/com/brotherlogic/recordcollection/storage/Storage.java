package com.brotherlogic.recordcollection.storage;

import com.brotherlogic.recordcollection.RecordCollection;

import java.util.List;
import java.util.Map;

import org.scribe.model.Token;

public interface Storage {
  void forceCleanDatabase();
  Token getToken(String userKey);
  void storeToken(Token token);
  void storeCollection(Integer userId, RecordCollection c);
  RecordCollection getCollection(Integer userId, String name);
  List<RecordCollection> getCollections(Integer userId);
}
