package com.brotherlogic.recordcollection;

import java.util.LinkedList;
import java.util.List;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;

public class FolderGroup {
  private List<Integer> folders = new LinkedList<Integer>();
  private List<Integer> proportions = new LinkedList<Integer>();
  private String name = "";

  public FolderGroup(JsonObject data) {
    for(JsonElement folder : data.get("folders").getAsJsonArray()) {
      folders.add(folder.getAsInt());
    }
    for(JsonElement prop : data.get("proportions").getAsJsonArray()) {
      proportions.add(prop.getAsInt());
    }
    name = data.get("name").getAsString();
  }
  
  public FolderGroup(List<Integer> folds, List<Integer> props, String nm) {
    folders.addAll(folds);
    proportions.addAll(props);
    name = nm;
  }

  public List<Integer> getFolders() {
    return folders;
  }

  public List<Integer> getProps() {
    return proportions;
  }

  public String getName() {
    return name;
  }
}
