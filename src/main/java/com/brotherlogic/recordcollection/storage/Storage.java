package com.brotherlogic.recordcollection.storage;

import com.brotherlogic.recordcollection.FolderGroup;

import java.util.List;
import java.util.Map;

import org.scribe.model.Token;

public interface Storage {
  void forceCleanDatabase();
  Token getToken(String userKey);
  void storeToken(Token token);
  void storeCollection(Integer userId, FolderGroup c);
  FolderGroup getCollection(Integer userId, String name);
  List<FolderGroup> getCollections(Integer userId);
}
